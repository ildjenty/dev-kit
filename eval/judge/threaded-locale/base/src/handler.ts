import * as service from "./service"
import { Body } from "./format"

export function handle(req: { body: Body; headers: Record<string, string> }) {
  return service.run(req.body)
}
