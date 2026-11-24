package acme.customer

object CustomerNames {
  fun display(c: Customer): String {
    val parts = listOfNotNull(c.familyName, c.givenName)
    return if (parts.isEmpty()) c.companyName ?: "" else parts.joinToString(" ")
  }
}
