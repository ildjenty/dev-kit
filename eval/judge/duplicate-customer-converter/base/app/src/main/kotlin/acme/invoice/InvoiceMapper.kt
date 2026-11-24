package acme.invoice

import acme.customer.Customer
import acme.customer.CustomerNames

object InvoiceMapper {
  fun recipient(c: Customer): String = CustomerNames.display(c)
}
