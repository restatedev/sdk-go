package main

import (
	"slices"
	"time"

	restate "github.com/restatedev/sdk-go"
)

const UserSessionServiceName = "UserSession"

type userSession struct{}

func (u *userSession) ServiceName() string {
	return UserSessionServiceName
}

func (u *userSession) AddTicket(ctx restate.ObjectContext, ticketId string) (bool, error) {
	userId := ctx.Key()

	success, err := restate.CallAs[bool](ctx.Object(TicketServiceName, ticketId, "Reserve")).Request(userId)
	if err != nil {
		return false, err
	}

	if !success {
		return false, nil
	}

	// add ticket to list of tickets
	tickets, err := restate.GetAs[[]string](ctx, "tickets")
	if err != nil {
		return false, err
	}

	tickets = append(tickets, ticketId)

	ctx.Set("tickets", tickets)
	ctx.Object(UserSessionServiceName, userId, "ExpireTicket").Send(ticketId, 15*time.Minute)

	return true, nil
}

func (u *userSession) ExpireTicket(ctx restate.ObjectContext, ticketId string) (void restate.Void, err error) {
	tickets, err := restate.GetAs[[]string](ctx, "tickets")
	if err != nil {
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

	ctx.Set("tickets", tickets)
	ctx.Object(TicketServiceName, ticketId, "Unreserve").Send(nil, 0)

	return void, nil
}

func (u *userSession) Checkout(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
	userId := ctx.Key()
	tickets, err := restate.GetAs[[]string](ctx, "tickets")
	if err != nil {
		return false, err
	}

	ctx.Log().Info("tickets in basket", "tickets", tickets)

	if len(tickets) == 0 {
		return false, nil
	}

	timeout := ctx.After(time.Minute)

	request := restate.CallAs[PaymentResponse](ctx.Object(CheckoutServiceName, "", "Payment")).
		RequestFuture(PaymentRequest{UserID: userId, Tickets: tickets})

	// race between the request and the timeout
	switch ctx.Select(timeout, request).Select() {
	case request:
		// happy path
	case timeout:
		// we could choose to fail here with terminal error, but we'd also have to refund the payment!
		ctx.Log().Warn("slow payment")
	}

	// block on the eventual response
	response, err := request.Response()
	if err != nil {
		return false, err
	}

	ctx.Log().Info("payment details", "id", response.ID, "price", response.Price)

	for _, ticket := range tickets {
		call := ctx.Object(TicketServiceName, ticket, "MarkAsSold")
		call.Send(nil, 0)
	}

	ctx.Clear("tickets")
	return true, nil
}
