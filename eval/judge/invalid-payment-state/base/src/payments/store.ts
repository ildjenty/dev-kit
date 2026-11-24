import { Payment } from "./types"

const payments = new Map<string, Payment>()
export function save(payment: Payment) { payments.set(payment.id, payment) }
export function load(id: string) { return payments.get(id) }
