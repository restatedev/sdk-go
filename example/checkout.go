package main

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/rs/zerolog/log"
)

type PaymentRequest struct {
	UserID  string
	Tickets []string
}

func payment(ctx restate.Context, request PaymentRequest) (bool, error) {
	uuid, err := restate.SideEffectAs(ctx, func() (string, error) {
		uuid := uuid.New()
		return uuid.String(), nil
	})

	if err != nil {
		return false, err
	}

	// We are a uniform shop where everything costs 30 USD
	// that is cheaper than the official example :P
	price := len(request.Tickets) * 30

	i := 0
	success, err := restate.SideEffectAs(ctx, func() (bool, error) {
		log := log.With().Str("uuid", uuid).Int("price", price).Logger()
		if i > 2 {
			log.Info().Msg("payment succeeded")
			return true, nil
		}

		log.Error().Msg("payment failed")
		i += 1
		return false, fmt.Errorf("failed to pay")
	})

	if err != nil {
		return false, err
	}

	// todo: send email

	return success, nil
}

var (
	Checkout = restate.NewUnKeyedRouter().
		Handler("checkout", restate.NewUnKeyedHandler(payment))
)
