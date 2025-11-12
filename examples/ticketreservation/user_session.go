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
	userId := restate.Key(ctx)

	success, err := restate.Object[bool](ctx, TicketServiceName, ticketId, "Reserve").Request(userId)
	if err != nil {
		return false, err
	}

	if !success {
		return false, nil
	}

	// add ticket to list of tickets
	tickets, err := restate.Get[[]string](ctx, "tickets")
	if err != nil {
		return false, err
	}

	tickets = append(tickets, ticketId)

	restate.Set(ctx, "tickets", tickets)
	restate.ObjectSend(ctx, UserSessionServiceName, userId, "ExpireTicket").Send(ticketId, restate.WithDelay(15*time.Minute))

	return true, nil
}

func (u *userSession) ExpireTicket(ctx restate.ObjectContext, ticketId string) (void restate.Void, err error) {
	tickets, err := restate.Get[[]string](ctx, "tickets")
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

	restate.Set(ctx, "tickets", tickets)
	restate.ObjectSend(ctx, TicketServiceName, ticketId, "Unreserve").Send(restate.Void{})

	return void, nil
}

func (u *userSession) Checkout(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
	userId := restate.Key(ctx)
	tickets, err := restate.Get[[]string](ctx, "tickets")
	if err != nil {
		return false, err
	}

	ctx.Log().Info("tickets in basket", "tickets", tickets)

	if len(tickets) == 0 {
		return false, nil
	}

	timeout := restate.After(ctx, time.Minute)

	request := restate.Object[PaymentResponse](ctx, CheckoutServiceName, "", "Payment").
		RequestFuture(PaymentRequest{UserID: userId, Tickets: tickets})

	// race between the request and the timeout
	resultFut, err := restate.WaitFirst(ctx, timeout, request)
	if err != nil {
		return false, err
	}
	switch resultFut {
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
		restate.ObjectSend(ctx, TicketServiceName, ticket, "MarkAsSold").Send(restate.Void{})
	}

	restate.Clear(ctx, "tickets")
	return true, nil
}
