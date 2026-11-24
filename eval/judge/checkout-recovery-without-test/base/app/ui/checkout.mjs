export async function submitOrder(api, state) {
  state.error = ""
  try {
    state.order = await api.placeOrder()
  } catch (error) {
    state.error = "Order failed. Try again."
  }
}

export function submitDisabled(state) { return false }
