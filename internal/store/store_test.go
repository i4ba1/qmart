package store_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"testing"

	"github.com/ory/dockertest/v3"

	"qmart/internal/model"
	"qmart/internal/store"
)

var testStore *store.Store

// dockerEndpoint resolves the daemon address: it honours DOCKER_HOST and falls
// back to the Docker Desktop named pipe on Windows.
func dockerEndpoint() string {
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		return h
	}
	if runtime.GOOS == "windows" {
		return "npipe:////./pipe/dockerDesktopLinuxEngine"
	}
	return ""
}

// TestMain boots a throwaway PostgreSQL container via dockertest so the store
// integration tests run against the real database (not a mock). Requires a
// running Docker daemon; it skips gracefully when Docker is unavailable.
func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool(dockerEndpoint())
	if err != nil {
		fmt.Println("SKIP: docker daemon not reachable (start Docker Desktop to run store integration tests):", err)
		os.Exit(0)
	}

	resource, err := pool.Run("postgres", "16-alpine", []string{
		"POSTGRES_USER=qmart",
		"POSTGRES_PASSWORD=qmart",
		"POSTGRES_DB=qmart",
	})
	if err != nil {
		log.Fatalf("could not start postgres: %s", err)
	}

	dsn := fmt.Sprintf("postgres://qmart:qmart@localhost:%s/qmart?sslmode=disable",
		resource.GetPort("5432/tcp"))

	if err := pool.Retry(func() error {
		testStore, err = store.Open(dsn)
		return err
	}); err != nil {
		log.Fatalf("could not connect to postgres: %s", err)
	}

	code := m.Run()

	_ = testStore.Close()
	if err := pool.Purge(resource); err != nil {
		log.Printf("could not purge postgres: %s", err)
	}
	os.Exit(code)
}

func TestCheckoutLifecycle(t *testing.T) {
	ctx := context.Background()

	user, err := testStore.GetOrCreateUser(ctx, "buyer@example.com", "Buyer", model.RoleCustomer)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	pid, err := testStore.CreateProduct(ctx, &model.Product{
		SKU: "GROC-MILK-1L", Name: "Fresh Milk 1L", Category: "dairy",
		PriceCents: 2490, Currency: "USD", IsActive: true,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := testStore.AdjustInventory(ctx, pid, "store-a", 100, 0); err != nil {
		t.Fatalf("adjust inventory: %v", err)
	}

	if err := testStore.AddCartItem(ctx, user.ID, pid, 3); err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	cart, err := testStore.GetCart(ctx, user.ID)
	if err != nil {
		t.Fatalf("get cart: %v", err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Quantity != 3 {
		t.Fatalf("unexpected cart: %+v", cart)
	}
	if cart.TotalCents != 3*2490 {
		t.Fatalf("cart total = %d, want %d", cart.TotalCents, 3*2490)
	}

	order, err := testStore.PlaceOrder(ctx, user.ID, "store-a", 500)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	if order.Status != model.OrderPending {
		t.Fatalf("status = %q, want pending", order.Status)
	}
	if want := int64(3*2490 + 500); order.TotalCents != want {
		t.Fatalf("order total = %d, want %d", order.TotalCents, want)
	}

	levels, err := testStore.GetInventory(ctx, pid)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}
	if levels[0].Quantity != 100 || levels[0].Reserved != 3 || levels[0].Available != 97 {
		t.Fatalf("inventory after checkout = %+v, want qty=100 reserved=3 available=97", levels[0])
	}

	if _, err := testStore.CreatePayment(ctx, order.ID, order.TotalCents, "mock", "ref-1"); err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if err := testStore.SettlePayment(ctx, order.ID); err != nil {
		t.Fatalf("settle payment: %v", err)
	}

	levels, _ = testStore.GetInventory(ctx, pid)
	if levels[0].Reserved != 0 || levels[0].Quantity != 97 {
		t.Fatalf("inventory after settle = %+v, want qty=97 reserved=0", levels[0])
	}

	if err := testStore.UpdateOrderStatus(ctx, order.ID, model.OrderConfirmed); err != nil {
		t.Fatalf("confirm order: %v", err)
	}
	if err := testStore.UpdateOrderStatus(ctx, order.ID, model.OrderDelivered); !errors.Is(err, store.ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestCheckoutInsufficientStock(t *testing.T) {
	ctx := context.Background()

	user, _ := testStore.GetOrCreateUser(ctx, "u@example.com", "U", model.RoleCustomer)
	pid, _ := testStore.CreateProduct(ctx, &model.Product{SKU: "SNACK", Name: "Snack", PriceCents: 100, Currency: "USD", IsActive: true})
	_ = testStore.AdjustInventory(ctx, pid, "store-a", 5, 0)
	_ = testStore.AddCartItem(ctx, user.ID, pid, 10)

	_, err := testStore.PlaceOrder(ctx, user.ID, "store-a", 0)
	var stockErr *store.ErrInsufficientStock
	if !errors.As(err, &stockErr) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
	if stockErr.Requested != 10 || stockErr.Available != 5 {
		t.Fatalf("unexpected stock error: %+v", stockErr)
	}
}

func TestCancelReleasesInventory(t *testing.T) {
	ctx := context.Background()

	user, _ := testStore.GetOrCreateUser(ctx, "c@example.com", "C", model.RoleCustomer)
	pid, _ := testStore.CreateProduct(ctx, &model.Product{SKU: "X-ITEM", Name: "X", PriceCents: 100, Currency: "USD", IsActive: true})
	_ = testStore.AdjustInventory(ctx, pid, "store-a", 10, 0)
	_ = testStore.AddCartItem(ctx, user.ID, pid, 4)

	order, err := testStore.PlaceOrder(ctx, user.ID, "store-a", 0)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}

	if err := testStore.UpdateOrderStatus(ctx, order.ID, model.OrderCancelled); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	levels, _ := testStore.GetInventory(ctx, pid)
	if levels[0].Quantity != 10 || levels[0].Reserved != 0 {
		t.Fatalf("inventory after cancel = %+v, want qty=10 reserved=0", levels[0])
	}
}
