package main

import (
	"errors"
	"slices"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/rs/zerolog/log"
)

func addTicket(ctx restate.ObjectContext, ticketId string) (bool, error) {
	userId := ctx.Key()

	var success bool
	if err := ctx.Object(TicketServiceName, ticketId).Method("reserve").Request(userId).Response(&success); err != nil {
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

	if err := ctx.ObjectSend(UserSessionServiceName, ticketId, 15*time.Minute).Method("expireTicket").Request(ticketId); err != nil {
		return false, err
	}

	return true, nil
}

func expireTicket(ctx restate.ObjectContext, ticketId string) (void restate.Void, err error) {
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

	return void, ctx.ObjectSend(TicketServiceName, ticketId, 0).Method("unreserve").Request(nil)
}

func checkout(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
	userId := ctx.Key()
	tickets, err := restate.GetAs[[]string](ctx, "tickets")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return false, err
	}

	log.Info().Strs("tickets", tickets).Msg("tickets in basket")

	if len(tickets) == 0 {
		return false, nil
	}

	var response PaymentResponse
	if err := ctx.Object(CheckoutServiceName, "").
		Method("checkout").
		Request(PaymentRequest{UserID: userId, Tickets: tickets}).
		Response(&response); err != nil {
		return false, err
	}

	log.Info().Str("id", response.ID).Int("price", response.Price).Msg("payment details")

	for _, ticket := range tickets {
		call := ctx.ObjectSend(TicketServiceName, ticket, 0).Method("markAsSold")
		if err := call.Request(nil); err != nil {
			return false, err
		}
	}

	ctx.Clear("tickets")
	return true, nil
}

var (
	UserSession = restate.NewObjectRouter().
		Handler("addTicket", restate.NewObjectHandler(addTicket)).
		Handler("expireTicket", restate.NewObjectHandler(expireTicket)).
		Handler("checkout", restate.NewObjectHandler(checkout))
)
