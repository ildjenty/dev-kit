import * as domain from "./domain"
import { Body } from "./format"

export function run(body: Body) { return domain.apply(body) }
