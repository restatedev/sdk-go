package main

import (
	"errors"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/rs/zerolog/log"
)

type TicketStatus int

const (
	TicketAvailable TicketStatus = 0
	TicketReserved  TicketStatus = 1
	TicketSold      TicketStatus = 2
)

func reserve(ctx restate.Context, ticketId string, _ restate.Void) (bool, error) {
	log.Info().Str("ticket", ticketId).Msg("reserving ticket")
	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return false, err
	}

	if status == TicketAvailable {
		return true, restate.SetAs(ctx, "status", TicketReserved)
	}

	return false, nil
}

func unreserve(ctx restate.Context, ticketId string, _ restate.Void) (void restate.Void, err error) {
	log.Info().Str("ticket", ticketId).Msg("un-reserving ticket")
	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return void, err
	}

	if status != TicketSold {
		return void, ctx.Clear("status")
	}

	return void, nil
}

func markAsSold(ctx restate.Context, ticketId string, _ restate.Void) (void restate.Void, err error) {
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

var (
	TicketService = restate.NewKeyedRouter().
		Handler("reserve", restate.NewKeyedHandler(reserve)).
		Handler("unreserve", restate.NewKeyedHandler(unreserve)).
		Handler("markAsSold", restate.NewKeyedHandler(markAsSold))
)
