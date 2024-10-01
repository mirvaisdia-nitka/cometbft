package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtpubsub "github.com/cometbft/cometbft/libs/pubsub"
	cmtquery "github.com/cometbft/cometbft/libs/pubsub/query"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/cometbft/cometbft/state/txindex"
	"github.com/cometbft/cometbft/state/txindex/null"
)

const (
	// maxQueryLength is the maximum length of a query string that will be
	// accepted. This is just a safety check to avoid outlandish queries.
	maxQueryLength = 512
)

type ErrParseQuery struct {
	Source error
}

func (e ErrParseQuery) Error() string {
	return fmt.Sprintf("failed to parse query: %v", e.Source)
}

func (e ErrParseQuery) Unwrap() error {
	return e.Source
}

// Subscribe for events via WebSocket.
// More: https://docs.cometbft.com/main/rpc/#/Websocket/subscribe
func (env *Environment) Subscribe(ctx *rpctypes.Context, query string) (*ctypes.ResultSubscribe, error) {
	addr := ctx.RemoteAddr()

	switch {
	case env.EventBus.NumClients() >= env.Config.MaxSubscriptionClients:
		return nil, ErrMaxSubscription{env.Config.MaxSubscriptionClients}
	case env.EventBus.NumClientSubscriptions(addr) >= env.Config.MaxSubscriptionsPerClient:
		return nil, ErrMaxPerClientSubscription{env.Config.MaxSubscriptionsPerClient}
	case len(query) > maxQueryLength:
		return nil, ErrQueryLength{len(query), maxQueryLength}
	}

	env.Logger.Info("Subscribe to query", "remote", addr, "query", query)

	q, err := cmtquery.New(query)
	if err != nil {
		return nil, ErrParseQuery{Source: err}
	}

	subCtx, cancel := context.WithTimeout(ctx.Context(), SubscribeTimeout)
	defer cancel()

	sub, err := env.EventBus.Subscribe(subCtx, addr, q, env.Config.SubscriptionBufferSize)
	if err != nil {
		return nil, err
	}

	closeIfSlow := env.Config.CloseOnSlowClient

	// Capture the current ID, since it can change in the future.
	subscriptionID := ctx.JSONReq.ID
	go func() {
		for {
			select {
			case msg := <-sub.Out():
				var (
					resultEvent = &ctypes.ResultEvent{Query: query, Data: msg.Data(), Events: msg.Events()}
					resp        = rpctypes.NewRPCSuccessResponse(subscriptionID, resultEvent)
				)
				writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := ctx.WSConn.WriteRPCResponse(writeCtx, resp); err != nil {
					env.Logger.Info("Can't write response (slow client)",
						"to", addr, "subscriptionID", subscriptionID, "err", err)

					if closeIfSlow {
						var (
							err  = ErrSubCanceled{ErrSlowClient.Error()}
							resp = rpctypes.RPCServerError(subscriptionID, err)
						)
						if !ctx.WSConn.TryWriteRPCResponse(resp) {
							env.Logger.Info("Can't write response (slow client)",
								"to", addr, "subscriptionID", subscriptionID, "err", err)
						}
						return
					}
				}
			case <-sub.Canceled():
				if !errors.Is(sub.Err(), cmtpubsub.ErrUnsubscribed) {
					var reason string
					if sub.Err() == nil {
						reason = ErrCometBFTExited.Error()
					} else {
						reason = sub.Err().Error()
					}
					var (
						err  = ErrSubCanceled{reason}
						resp = rpctypes.RPCServerError(subscriptionID, err)
					)
					if !ctx.WSConn.TryWriteRPCResponse(resp) {
						env.Logger.Info("Can't write response (slow client)",
							"to", addr, "subscriptionID", subscriptionID, "err", err)
					}
				}
				return
			}
		}
	}()

	return &ctypes.ResultSubscribe{}, nil
}

// Unsubscribe from events via WebSocket.
// More: https://docs.cometbft.com/main/rpc/#/Websocket/unsubscribe
func (env *Environment) Unsubscribe(ctx *rpctypes.Context, query string) (*ctypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	env.Logger.Info("Unsubscribe from query", "remote", addr, "query", query)
	q, err := cmtquery.New(query)
	if err != nil {
		return nil, ErrParseQuery{Source: err}
	}

	err = env.EventBus.Unsubscribe(context.Background(), addr, q)
	if err != nil {
		return nil, err
	}

	return &ctypes.ResultUnsubscribe{}, nil
}

// UnsubscribeAll from all events via WebSocket.
// More: https://docs.cometbft.com/main/rpc/#/Websocket/unsubscribe_all
func (env *Environment) UnsubscribeAll(ctx *rpctypes.Context) (*ctypes.ResultUnsubscribe, error) {
	addr := ctx.RemoteAddr()
	env.Logger.Info("Unsubscribe from all", "remote", addr)
	err := env.EventBus.UnsubscribeAll(context.Background(), addr)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
}

// EventSearch allows you to query for events across blocks. It returns a
// list of transaction events and block events (maximum ?per_page entries) and the total count.
// More: https://docs.cometbft.com/main/rpc/#/Info/event_search
func (env *Environment) EventSearch(
	ctx *rpctypes.Context,
	query string,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*ctypes.ResultEventSearch, error) {
	// if index is disabled, return error
	if _, ok := env.TxIndexer.(*null.TxIndex); ok {
		return nil, ErrTxIndexingDisabled
	} else if len(query) > maxQueryLength {
		return nil, ErrQueryLength{len(query), maxQueryLength}
	}

	// if orderBy is not "asc", "desc", or blank, return error
	if orderBy != "" && orderBy != Ascending && orderBy != Descending {
		return nil, ErrInvalidOrderBy{orderBy}
	}

	q, err := cmtquery.New(query)
	if err != nil {
		return nil, err
	}

	// Validate number of results per page
	perPage := env.validatePerPage(perPagePtr)
	if pagePtr == nil {
		// Default to page 1 if not specified
		pagePtr = new(int)
		*pagePtr = 1
	}

	pagSettings := txindex.Pagination{
		OrderDesc:   orderBy == Descending,
		IsPaginated: true,
		Page:        *pagePtr,
		PerPage:     perPage,
	}

	txsBlockEvents := make([]ctypes.ResultTxsBlockEvents, 0)

	// Retrieve the txs results events
	txResults, totalCount, err := env.TxIndexer.Search(ctx.Context(), q, pagSettings)
	if err != nil {
		return nil, err
	}

	for _, r := range txResults {
		events := make([]abci.Event, 0)

		events = append(events, r.Result.Events...)

		txsBlockEvents = append(txsBlockEvents, ctypes.ResultTxsBlockEvents{
			Height: r.Height,
			Events: events,
		})
	}

	return &ctypes.ResultEventSearch{ResultEvents: txsBlockEvents, TotalCount: totalCount}, nil
}
