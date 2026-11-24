import { Cart, cartTotal } from "./pricing"
import { charge } from "./gateway"

export function checkout(cart: Cart) {
  return charge(cart.user, cartTotal(cart))
}
