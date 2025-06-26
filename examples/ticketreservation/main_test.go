package main

import (
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/mocks"
	"github.com/stretchr/testify/assert"
)

func TestPayment(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().MockRand().UUID().Return(uuid.Max)

	mockCtx.EXPECT().RunAndExpect(mockCtx, true, nil)
	mockCtx.EXPECT().Log().Return(slog.Default())

	resp, err := (&checkout{}).Payment(restate.WithMockContext(mockCtx), PaymentRequest{Tickets: []string{"abc"}})
	assert.NoError(t, err)
	assert.Equal(t, resp, PaymentResponse{ID: "ffffffff-ffff-ffff-ffff-ffffffffffff", Price: 30})
}

func TestReserve(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().GetAndReturn("status", TicketAvailable)
	mockCtx.EXPECT().Set("status", TicketReserved)

	ok, err := (&ticketService{}).Reserve(restate.WithMockContext(mockCtx), restate.Void{})
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestUnreserve(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().Key().Return("foo")
	mockCtx.EXPECT().Log().Return(slog.Default())
	mockCtx.EXPECT().GetAndReturn("status", TicketAvailable)
	mockCtx.EXPECT().Clear("status")

	_, err := (&ticketService{}).Unreserve(restate.WithMockContext(mockCtx), restate.Void{})
	assert.NoError(t, err)
}

func TestMarkAsSold(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().Key().Return("foo")
	mockCtx.EXPECT().Log().Return(slog.Default())
	mockCtx.EXPECT().GetAndReturn("status", TicketReserved)
	mockCtx.EXPECT().Set("status", TicketSold)

	_, err := (&ticketService{}).MarkAsSold(restate.WithMockContext(mockCtx), restate.Void{})
	assert.NoError(t, err)
}

func TestStatus(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().Key().Return("foo")
	mockCtx.EXPECT().Log().Return(slog.Default())
	mockCtx.EXPECT().GetAndReturn("status", TicketReserved)

	status, err := (&ticketService{}).Status(restate.WithMockContext(mockCtx), restate.Void{})
	assert.NoError(t, err)
	assert.Equal(t, status, TicketReserved)
}

func TestAddTicket(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().Key().Return("userID")
	mockCtx.EXPECT().MockObjectClient(TicketServiceName, "ticket2", "Reserve").RequestAndReturn("userID", true, nil)

	mockCtx.EXPECT().GetAndReturn("tickets", []string{"ticket1"})
	mockCtx.EXPECT().Set("tickets", []string{"ticket1", "ticket2"})
	mockCtx.EXPECT().MockObjectClient(UserSessionServiceName, "userID", "ExpireTicket").
		MockSend("ticket2", restate.WithDelay(15*time.Minute))

	ok, err := (&userSession{}).AddTicket(restate.WithMockContext(mockCtx), "ticket2")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestExpireTicket(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().GetAndReturn("tickets", []string{"ticket1", "ticket2"})
	mockCtx.EXPECT().Set("tickets", []string{"ticket1"})

	mockCtx.EXPECT().MockObjectClient(TicketServiceName, "ticket2", "Unreserve").MockSend(restate.Void{})

	_, err := (&userSession{}).ExpireTicket(restate.WithMockContext(mockCtx), "ticket2")
	assert.NoError(t, err)
}

func TestCheckout(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().Key().Return("userID")
	mockCtx.EXPECT().GetAndReturn("tickets", []string{"ticket1"})
	mockCtx.EXPECT().Log().Return(slog.Default())

	mockAfter := mockCtx.EXPECT().MockAfter(time.Minute)

	mockResponseFuture := mockCtx.EXPECT().MockObjectClient(CheckoutServiceName, "", "Payment").
		MockResponseFuture(PaymentRequest{UserID: "userID", Tickets: []string{"ticket1"}})

	mockCtx.EXPECT().MockSelector(mockAfter, mockResponseFuture).
		Select().Return(mockResponseFuture)

	mockResponseFuture.EXPECT().ResponseAndReturn(PaymentResponse{ID: "paymentID", Price: 30}, nil)

	mockCtx.EXPECT().MockObjectClient(TicketServiceName, "ticket1", "MarkAsSold").MockSend(restate.Void{})

	mockCtx.EXPECT().Clear("tickets")

	ok, err := (&userSession{}).Checkout(restate.WithMockContext(mockCtx), restate.Void{})
	assert.NoError(t, err)
	assert.True(t, ok)
}
