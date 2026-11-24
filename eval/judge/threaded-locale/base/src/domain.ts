import { format, Body } from "./format"
import * as repo from "./repo"

export function apply(body: Body) { return repo.save(format(body)) }
