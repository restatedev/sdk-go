package main

import (
	"errors"
	"slices"
	"time"

	restate "github.com/restatedev/sdk-go"
)

const UserSessionServiceName = "UserSession"

type userSession struct{}

func (u *userSession) Name() string {
	return UserSessionServiceName
}

func (u *userSession) AddTicket(ctx restate.ObjectContext, ticketId string) (bool, error) {
	userId := ctx.Key()

	var success bool
	if err := ctx.Object(TicketServiceName, ticketId).Method("Reserve").Request(userId).Response(&success); err != nil {
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

	if err := ctx.ObjectSend(UserSessionServiceName, ticketId, 15*time.Minute).Method("ExpireTicket").Request(ticketId); err != nil {
		return false, err
	}

	return true, nil
}

func (u *userSession) ExpireTicket(ctx restate.ObjectContext, ticketId string) (void restate.Void, err error) {
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

	return void, ctx.ObjectSend(TicketServiceName, ticketId, 0).Method("Unreserve").Request(nil)
}

func (u *userSession) Checkout(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
	userId := ctx.Key()
	tickets, err := restate.GetAs[[]string](ctx, "tickets")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return false, err
	}

	ctx.Log().Info("tickets in basket", "tickets", tickets)

	if len(tickets) == 0 {
		return false, nil
	}

	var response PaymentResponse
	if err := ctx.Object(CheckoutServiceName, "").
		Method("Payment").
		Request(PaymentRequest{UserID: userId, Tickets: tickets}).
		Response(&response); err != nil {
		return false, err
	}

	ctx.Log().Info("payment details", "id", response.ID, "price", response.Price)

	for _, ticket := range tickets {
		call := ctx.ObjectSend(TicketServiceName, ticket, 0).Method("MarkAsSold")
		if err := call.Request(nil); err != nil {
			return false, err
		}
	}

	ctx.Clear("tickets")
	return true, nil
}
