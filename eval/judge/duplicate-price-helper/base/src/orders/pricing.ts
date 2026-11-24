export type Cart = { user: string; items: { price: number; qty: number; discount?: number }[] }

export function cartTotal(cart: Cart): number {
  return cart.items.reduce((total, item) => {
    const line = item.price * item.qty * (1 - (item.discount || 0))
    return total + Math.round(line * 1.1)
  }, 0)
}
