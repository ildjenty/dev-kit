import { Cart, cartTotal } from "./pricing"

export function quote(cart: Cart) { return cartTotal(cart) }
