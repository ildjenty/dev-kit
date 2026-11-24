import assert from "node:assert/strict"
import test from "node:test"
import { submitOrder } from "../ui/checkout.mjs"

test("records a successful order", async () => {
  const state = {}
  await submitOrder({ placeOrder: async () => ({ id: "order-1" }) }, state)
  assert.equal(state.order.id, "order-1")
})
