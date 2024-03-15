package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/muhamadazmy/restate-sdk-go/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type J = map[string]interface{}

type Tickets struct{}

func (t *Tickets) Reserve(ctx restate.Context, id string, _ restate.Void) (restate.Void, error) {
	if err := ctx.Set("reserved", []byte{1}); err != nil {
		return restate.Void{}, err
	}

	if err := ctx.Service("Tickets").Method("UnReserve").Send(id, nil, 10*time.Second); err != nil {
		return restate.Void{}, err
	}

	return restate.Void{}, nil
}

func (t *Tickets) UnReserve(ctx restate.Context, id string, _ restate.Void) (restate.Void, error) {
	if err := ctx.Clear("reserved"); err != nil {
		return restate.Void{}, err
	}

	log.Info().Msg("tick unreserved")

	return restate.Void{}, nil
}

func Echo(ctx restate.Context, name string) (string, error) {
	response, err := ctx.Service("Keyed").Method("SayHi").Do(name, J{})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("echo: %s", string(response)), nil
}

func SayHi(ctx restate.Context, key string, _ restate.Void) (string, error) {
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

func Keys(ctx restate.Context, key string, _ restate.Void) (restate.Void, error) {

	for i := 0; i < 100; i++ {
		ctx.Set(fmt.Sprintf("key.%d", i), []byte("value"))
	}

	return restate.Void{}, nil
}

func Sleep(ctx restate.Context, seconds uint64) (restate.Void, error) {
	log.Info().Uint64("seconds", seconds).Msg("sleeping for")

	return restate.Void{}, ctx.Sleep(time.Now().Add(time.Duration(seconds) * time.Second))
}

func main() {

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	var tickets Tickets

	r := server.NewRestate()

	ticketsService := restate.NewKeyedRouter().
		Handler("Reserve", restate.NewKeyedHandler(tickets.Reserve)).
		Handler("UnReserve", restate.NewKeyedHandler(tickets.UnReserve))

	r.Bind("Tickets", ticketsService)

	unKeyed := restate.NewUnKeyedRouter().
		Handler("Echo", restate.NewUnKeyedHandler(Echo)).
		Handler("Sleep", restate.NewUnKeyedHandler(Sleep))

	keyed := restate.NewKeyedRouter().
		Handler("SayHi", restate.NewKeyedHandler(SayHi)).
		Handler("Keys", restate.NewKeyedHandler(Keys))

	r.
		Bind("UnKeyed", unKeyed).
		Bind("Keyed", keyed)

	if err := r.Start(context.Background(), ":9080"); err != nil {
		panic(err)
	}
}
