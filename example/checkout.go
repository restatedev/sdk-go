package main

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/restatedev/restate-sdk-go"
	"github.com/rs/zerolog/log"
)

type PaymentRequest struct {
	UserID  string   `json:"userId"`
	Tickets []string `json:"tickets"`
}

type PaymentResponse struct {
	ID    string `json:"id"`
	Price int    `json:"price"`
}

func payment(ctx restate.Context, request PaymentRequest) (response PaymentResponse, err error) {
	uuid, err := restate.SideEffectAs(ctx, func() (string, error) {
		uuid := uuid.New()
		return uuid.String(), nil
	})

	response.ID = uuid

	if err != nil {
		return response, err
	}

	// We are a uniform shop where everything costs 30 USD
	// that is cheaper than the official example :P
	price := len(request.Tickets) * 30

	response.Price = price
	i := 0
	_, err = restate.SideEffectAs(ctx, func() (bool, error) {
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
		return response, err
	}

	// todo: send email

	return response, nil
}

var (
	Checkout = restate.NewUnKeyedRouter().
		Handler("checkout", restate.NewUnKeyedHandler(payment))
)
