package main

import (
	"context"
	"fmt"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/client"
)

func main() {
	ctx, err := client.Connect(context.Background(), "http://127.0.0.1:8080")
	if err != nil {
		panic(err)
	}

	if ok, err := AddTicketSend(ctx, "user-1", "ticket-1"); err != nil {
		panic(err)
	} else if !ok {
		fmt.Println("Ticket-1 was not available")
	} else {
		fmt.Println("Added ticket-1 to user-1 basket")
	}

	if ok, err := Checkout(ctx, "user-1", "ticket-1"); err != nil {
		panic(err)
	} else if !ok {
		fmt.Println("Nothing to check out")
	} else {
		fmt.Println("Checked out")
	}
}

func AddTicket(ctx context.Context, userId, ticketId string) (bool, error) {
	return client.
		Object[bool](ctx, "UserSession", userId, "AddTicket").
		Request(ticketId)
}

func AddTicketSend(ctx context.Context, userId, ticketId string) (bool, error) {
	send, err := client.
		Object[bool](ctx, "UserSession", userId, "AddTicket").
		Send(ticketId, restate.WithIdempotencyKey(fmt.Sprintf("%s/%s", userId, ticketId)))
	if err != nil {
		return false, err
	}

	fmt.Println("Submitted AddTicket with ID", send.InvocationId)

	o, ready, err := client.GetOutput[bool](ctx, send)
	if err != nil {
		return false, err
	}
	if ready {
		return o, nil
	}

	return client.Attach[bool](ctx, send)
}

func Checkout(ctx context.Context, userId, ticketId string) (bool, error) {
	return client.
		Object[bool](ctx, "UserSession", userId, "Checkout").
		Request(restate.Void{})
}
