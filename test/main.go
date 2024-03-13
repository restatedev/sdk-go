package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/muhamadazmy/restate-sdk-go/router"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type J = map[string]interface{}

type Tickets struct{}

func (t *Tickets) Reserve(ctx router.Context, id string, _ router.Void) (router.Void, error) {
	if err := ctx.Set("reserved", []byte{1}); err != nil {
		return router.Void{}, err
	}

	if err := ctx.Service("Tickets").Method("UnReserve").Send(id, nil, 10*time.Second); err != nil {
		return router.Void{}, err
	}

	return router.Void{}, nil
}

func (t *Tickets) UnReserve(ctx router.Context, id string, _ router.Void) (router.Void, error) {
	if err := ctx.Clear("reserved"); err != nil {
		return router.Void{}, err
	}

	log.Info().Msg("tick unreserved")

	return router.Void{}, nil
}

func Echo(ctx router.Context, name string) (string, error) {
	response, err := ctx.Service("Keyed").Method("SayHi").Do(name, J{})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("echo: %s", string(response)), nil
}

func SayHi(ctx router.Context, key string, _ router.Void) (string, error) {
	data, err := ctx.Get("count")
	if err != nil {
		return "", err
	}

	var count uint64
	if data != nil {
		if err := json.Unmarshal(data, &count); err != nil {
			return "", err
		}
	}

	if count > 5 {
		ctx.Clear("count")
		return "flushed", nil
	}
	count += 1
	if err := ctx.Set("count", []byte(fmt.Sprint(count))); err != nil {
		return "", err
	}

	return fmt.Sprintf("Hi: %s (%d)", key, count), nil
}

func Keys(ctx router.Context, key string, _ router.Void) (router.Void, error) {

	for i := 0; i < 100; i++ {
		ctx.Set(fmt.Sprintf("key.%d", i), []byte("value"))
	}

	return router.Void{}, nil
}

func Sleep(ctx router.Context, seconds uint64) (router.Void, error) {
	log.Info().Uint64("seconds", seconds).Msg("sleeping for")

	return router.Void{}, ctx.Sleep(time.Now().Add(time.Duration(seconds) * time.Second))
}

func main() {

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	var tickets Tickets

	r := restate.NewRestate()

	ticketsService := router.NewKeyedRouter().
		Handler("Reserve", router.NewKeyedHandler(tickets.Reserve)).
		Handler("UnReserve", router.NewKeyedHandler(tickets.UnReserve))

	r.Bind("Tickets", ticketsService)

	unKeyed := router.NewUnKeyedRouter().
		Handler("Echo", router.NewUnKeyedHandler(Echo)).
		Handler("Sleep", router.NewUnKeyedHandler(Sleep))

	keyed := router.NewKeyedRouter().
		Handler("SayHi", router.NewKeyedHandler(SayHi)).
		Handler("Keys", router.NewKeyedHandler(Keys))

	r.
		Bind("UnKeyed", unKeyed).
		Bind("Keyed", keyed)

	if err := r.Start(context.Background(), ":9080"); err != nil {
		panic(err)
	}
}
