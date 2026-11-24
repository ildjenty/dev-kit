export type PaymentState =
  | { status: "pending" }
  | { status: "paid"; paidAt: Date }
  | { status: "failed"; failedReason: string }
  | { status: "refunded"; refundedAt: Date }

export type Payment = { id: string; state: PaymentState }
