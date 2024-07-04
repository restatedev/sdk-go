package main

import (
	"errors"
	"slices"
	"time"

	"github.com/restatedev/restate-sdk-go"
	"github.com/rs/zerolog/log"
)

func addTicket(ctx restate.Context, userId, ticketId string) (bool, error) {

	var success bool
	if err := ctx.Service(TicketServiceName).Method("reserve").Do(ticketId, userId, &success); err != nil {
		return false, err
	}

	if !success {
		return false, nil
	}

	// add ticket to list of tickets
	tickets, err := restate.GetAs[[]string](ctx, "tickets")

	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return false, err
	}

	tickets = append(tickets, ticketId)

	if err := restate.SetAs(ctx, "tickets", tickets); err != nil {
		return false, err
	}

	if err := ctx.Service(UserSessionServiceName).Method("expireTicket").Send(userId, ticketId, 15*time.Minute); err != nil {
		return false, err
	}

	return true, nil
}

func expireTicket(ctx restate.Context, _, ticketId string) (void restate.Void, err error) {
	tickets, err := restate.GetAs[[]string](ctx, "tickets")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return void, err
	}

	deleted := false
	tickets = slices.DeleteFunc(tickets, func(ticket string) bool {
		if ticket == ticketId {
			deleted = true
			return true
		}
		return false
	})
	if !deleted {
		return void, nil
	}

	if err := restate.SetAs(ctx, "tickets", tickets); err != nil {
		return void, err
	}

	return void, ctx.Service(TicketServiceName).Method("unreserve").Send(ticketId, nil, 0)
}

func checkout(ctx restate.Context, userId string, _ restate.Void) (bool, error) {
	tickets, err := restate.GetAs[[]string](ctx, "tickets")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return false, err
	}

	log.Info().Strs("tickets", tickets).Msg("tickets in basket")

	if len(tickets) == 0 {
		return false, nil
	}

	var response PaymentResponse
	if err := ctx.Service(CheckoutServiceName).
		Method("checkout").
		Do("", PaymentRequest{UserID: userId, Tickets: tickets}, &response); err != nil {
		return false, err
	}

	log.Info().Str("id", response.ID).Int("price", response.Price).Msg("payment details")

	call := ctx.Service(TicketServiceName).Method("markAsSold")
	for _, ticket := range tickets {
		if err := call.Send(ticket, nil, 0); err != nil {
			return false, err
		}
	}

	return true, ctx.Clear("tickets")
}

var (
	UserSession = restate.NewKeyedRouter().
		Handler("addTicket", restate.NewKeyedHandler(addTicket)).
		Handler("expireTicket", restate.NewKeyedHandler(expireTicket)).
		Handler("checkout", restate.NewKeyedHandler(checkout))
)
