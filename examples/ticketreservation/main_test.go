package main

import (
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPayment(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)
	mockRand := mocks.NewMockRand(t)

	mockRand.EXPECT().UUID().Return(uuid.UUID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))

	mockCtx.EXPECT().Rand().Return(mockRand)
	mockCtx.EXPECT().RunAndExpect(t, mockCtx, true, nil)
	mockCtx.EXPECT().Log().Return(slog.Default())

	resp, err := (&checkout{}).Payment(restate.WithMockContext(mockCtx), PaymentRequest{Tickets: []string{"abc"}})
	assert.NoError(t, err)
	assert.Equal(t, resp, PaymentResponse{ID: "01020304-0506-0708-090a-0b0c0d0e0f10", Price: 30})
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
	mockTicketClient := mocks.NewMockClient(t)
	mockSessionClient := mocks.NewMockClient(t)

	mockCtx.EXPECT().Key().Return("userID")
	mockCtx.EXPECT().Object(TicketServiceName, "ticket2", "Reserve").Once().Return(mockTicketClient)
	mockTicketClient.EXPECT().RequestAndReturn("userID", true, nil)

	mockCtx.EXPECT().GetAndReturn("tickets", []string{"ticket1"})
	mockCtx.EXPECT().Set("tickets", []string{"ticket1", "ticket2"})
	mockCtx.EXPECT().Object(UserSessionServiceName, "userID", "ExpireTicket").Once().Return(mockSessionClient)
	mockSessionClient.EXPECT().Send("ticket2", restate.WithDelay(15*time.Minute))

	ok, err := (&userSession{}).AddTicket(restate.WithMockContext(mockCtx), "ticket2")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestExpireTicket(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)
	mockTicketClient := mocks.NewMockClient(t)

	mockCtx.EXPECT().GetAndReturn("tickets", []string{"ticket1", "ticket2"})
	mockCtx.EXPECT().Set("tickets", []string{"ticket1"})

	mockCtx.EXPECT().Object(TicketServiceName, "ticket2", "Unreserve").Once().Return(mockTicketClient)
	mockTicketClient.EXPECT().Send(restate.Void{})

	_, err := (&userSession{}).ExpireTicket(restate.WithMockContext(mockCtx), "ticket2")
	assert.NoError(t, err)
}

func TestCheckout(t *testing.T) {
	mockCtx := mocks.NewMockContext(t)

	mockCtx.EXPECT().Key().Return("userID")
	mockCtx.EXPECT().GetAndReturn("tickets", []string{"ticket1"})
	mockCtx.EXPECT().Log().Return(slog.Default())

	mockAfter := mocks.NewMockAfterFuture(t)
	mockCtx.EXPECT().After(time.Minute).Return(mockAfter)

	mockCheckoutClient := mocks.NewMockClient(t)
	mockCtx.EXPECT().Object(CheckoutServiceName, "", "Payment").Once().Return(mockCheckoutClient)
	mockResponseFuture := mocks.NewMockResponseFuture(t)
	mockCheckoutClient.EXPECT().RequestFuture(PaymentRequest{UserID: "userID", Tickets: []string{"ticket1"}}).Return(mockResponseFuture)

	mockSelector := mocks.NewMockSelector(t)
	mockCtx.EXPECT().Select(mockAfter, mock.AnythingOfType("restate.responseFuture[github.com/restatedev/sdk-go/examples/ticketreservation.PaymentResponse]")).Return(mockSelector)
	mockSelector.EXPECT().Select().Return(mockResponseFuture)

	mockResponseFuture.EXPECT().ResponseAndReturn(PaymentResponse{ID: "paymentID", Price: 30}, nil)

	mockTicketClient := mocks.NewMockClient(t)
	mockCtx.EXPECT().Object(TicketServiceName, "ticket1", "MarkAsSold").Once().Return(mockTicketClient)
	mockTicketClient.EXPECT().Send(restate.Void{})

	mockCtx.EXPECT().Clear("tickets")

	ok, err := (&userSession{}).Checkout(restate.WithMockContext(mockCtx), restate.Void{})
	assert.NoError(t, err)
	assert.True(t, ok)
}
