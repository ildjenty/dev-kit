import { submitOrder, submitDisabled } from "./checkout.mjs"

export async function onSubmit(api, state, button, message) {
  const pending = submitOrder(api, state)
  button.disabled = submitDisabled(state)
  await pending
  message.textContent = state.error
  button.disabled = submitDisabled(state)
}
