export type Body = { amount: number }

export function format(body: Body) { return String(body.amount) }
