package main

import (
	"errors"

	restate "github.com/restatedev/sdk-go"
	"github.com/rs/zerolog/log"
)

type TicketStatus int

const (
	TicketAvailable TicketStatus = 0
	TicketReserved  TicketStatus = 1
	TicketSold      TicketStatus = 2
)

const TicketServiceName = "TicketService"

type ticketService struct{}

func (t *ticketService) Name() string { return TicketServiceName }

func (t *ticketService) Reserve(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return false, err
	}

	if status == TicketAvailable {
		return true, restate.SetAs(ctx, "status", TicketReserved)
	}

	return false, nil
}

func (t *ticketService) Unreserve(ctx restate.ObjectContext, _ restate.Void) (void restate.Void, err error) {
	ticketId := ctx.Key()
	log.Info().Str("ticket", ticketId).Msg("un-reserving ticket")
	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return void, err
	}

	if status != TicketSold {
		ctx.Clear("status")
		return void, nil
	}

	return void, nil
}

func (t *ticketService) MarkAsSold(ctx restate.ObjectContext, _ restate.Void) (void restate.Void, err error) {
	ticketId := ctx.Key()
	log.Info().Str("ticket", ticketId).Msg("mark ticket as sold")

	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return void, err
	}

	if status == TicketReserved {
		return void, restate.SetAs(ctx, "status", TicketSold)
	}

	return void, nil
}
