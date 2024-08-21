package main

import (
	restate "github.com/restatedev/sdk-go"
)

type TicketStatus int

const (
	TicketAvailable TicketStatus = 0
	TicketReserved  TicketStatus = 1
	TicketSold      TicketStatus = 2
)

const TicketServiceName = "TicketService"

type ticketService struct{}

func (t *ticketService) ServiceName() string { return TicketServiceName }

func (t *ticketService) Reserve(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
	status, err := restate.Get[TicketStatus](ctx, "status")
	if err != nil {
		return false, err
	}

	if status == TicketAvailable {
		restate.Set(ctx, "status", TicketReserved)
		return true, nil
	}

	return false, nil
}

func (t *ticketService) Unreserve(ctx restate.ObjectContext, _ restate.Void) (void restate.Void, err error) {
	ticketId := restate.Key(ctx)
	ctx.Log().Info("un-reserving ticket", "ticket", ticketId)
	status, err := restate.Get[TicketStatus](ctx, "status")
	if err != nil {
		return void, err
	}

	if status != TicketSold {
		restate.Clear(ctx, "status")
		return void, nil
	}

	return void, nil
}

func (t *ticketService) MarkAsSold(ctx restate.ObjectContext, _ restate.Void) (void restate.Void, err error) {
	ticketId := restate.Key(ctx)
	ctx.Log().Info("mark ticket as sold", "ticket", ticketId)

	status, err := restate.Get[TicketStatus](ctx, "status")
	if err != nil {
		return void, err
	}

	if status == TicketReserved {
		restate.Set(ctx, "status", TicketSold)
		return void, nil
	}

	return void, nil
}

func (t *ticketService) Status(ctx restate.ObjectSharedContext, _ restate.Void) (TicketStatus, error) {
	ticketId := restate.Key(ctx)
	ctx.Log().Info("mark ticket as sold", "ticket", ticketId)

	return restate.Get[TicketStatus](ctx, "status")
}
