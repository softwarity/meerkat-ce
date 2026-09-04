// One-line, LOCALIZED descriptions for the add-menus (predicates and
// modifiers): the server catalog's docs are technical English, these are what
// a reader scans to pick the right brick. Keep them short - they render as
// the second line of a menu item. No curly braces here (ICU would eat them).

const DOCS: Record<string, string> = {
  // predicates
  path: $localize`:@@doc_Match_the_request_path:Match the request path; several patterns act as a logical OR`,
  host: $localize`:@@doc_Match_the_Host_header:Match the Host header (*.suffix wildcards)`,
  method: $localize`:@@doc_Match_the_HTTP_method:Match the HTTP method`,
  header: $localize`:@@doc_A_header_is_present:A header is present, optionally matching a regexp`,
  cookie: $localize`:@@doc_A_cookie_is_present:A cookie is present, optionally matching a regexp`,
  query: $localize`:@@doc_A_query_parameter_is_present:A query parameter is present, optionally matching a regexp`,
  'remote-addr': $localize`:@@doc_The_client_address_is_in_CIDR:The client address is in CIDR ranges`,
  'x-forwarded-remote-addr': $localize`:@@doc_The_last_XFF_address_is_in_CIDR:The last X-Forwarded-For address is in CIDR ranges`,
  after: $localize`:@@doc_Requests_made_after:Requests made after a datetime`,
  before: $localize`:@@doc_Requests_made_before:Requests made before a datetime`,
  between: $localize`:@@doc_Requests_made_between:Requests made between two datetimes`,
  weight: $localize`:@@doc_Split_traffic_canary:Split traffic between the routes of a group (canary)`,
  // modifiers
  'strip-prefix': $localize`:@@doc_Remove_first_path_segments:Remove the first N path segments`,
  'prefix-path': $localize`:@@doc_Prepend_a_prefix:Prepend a prefix to the path`,
  'rewrite-path': $localize`:@@doc_Rewrite_the_path_regexp:Rewrite the path with a regexp`,
  'set-request-header': $localize`:@@doc_Set_a_request_header:Set a request header, replacing the client value`,
  'add-request-header': $localize`:@@doc_Add_a_request_header:Add a request header value`,
  'remove-request-header': $localize`:@@doc_Remove_a_request_header:Remove a request header`,
  'set-query-param': $localize`:@@doc_Set_a_query_parameter:Set a query parameter`,
  'remove-query-param': $localize`:@@doc_Remove_a_query_parameter:Remove a query parameter`,
  'set-response-header': $localize`:@@doc_Set_a_response_header:Set a response header, replacing the upstream value`,
  'add-response-header': $localize`:@@doc_Add_a_response_header:Add a response header value`,
  'remove-response-header': $localize`:@@doc_Remove_a_response_header:Remove a response header`,
  'set-status': $localize`:@@doc_Force_the_response_status:Force the response status code`,
  redirect: $localize`:@@doc_Answer_with_a_redirect:Answer with a redirect instead of proxying`,
  maintenance: $localize`:@@doc_Answer_503_unavailable:Answer 503 with the gateway's unavailable page`,
};

export function brickDoc(type: string): string {
  return DOCS[type] ?? '';
}

// The bricks on the ROADMAP (mostly the missing Spring Cloud Gateway
// factories): shown grayed in the add drawer so what exists and what is
// coming reads in one place.
export interface PlannedBrick {
  type: string;
  doc: string;
}

// What is NOT here matters as much as what is: an entry that ships must leave
// this list, and the palette filters it against the live catalog on top of
// that (see FiltersComponent.planned). A hand-kept list of promises goes stale
// silently - three of these were sitting greyed out as "coming" while the
// server had been serving them for weeks, under a name of their own.
// Predicates have their own list. What is announced here is the RANGE, not
// version routing as such: the header predicate already matches a named
// version exactly, and what it cannot do is compare - a regexp compares
// characters, so 1.10 reads as smaller than 1.9. Naming every version instead
// is a list that rots at each release.
// Empty: version shipped. The palette filters this against the live catalog
// anyway, but an entry that shipped leaves the list - a promise kept twice is
// how the list rotted the first time.
export const PLANNED_PREDICATES: PlannedBrick[] = [];

export const PLANNED_MODIFIERS: Record<string, PlannedBrick[]> = {
  // Gates - the bricks that accept or refuse. All three of these REFUSE, which
  // is why they sit here and not among the modifiers, and all three wait on the
  // same thing: state that survives between requests (ROUTE-08/09).
  gate: [
    { type: 'rate-limiter', doc: $localize`:@@planned_rate_limiter:Throttle callers by key` },
    { type: 'circuit-breaker', doc: $localize`:@@planned_circuit_breaker:Stop calling a failing service, with a fallback` },
    { type: 'retry', doc: $localize`:@@planned_retry:Retry idempotent requests on upstream errors` },
  ],
  request: [
    {
      type: 'trust-forwarded-for',
      doc: $localize`:@@planned_trust_forwarded_for:Keep the X-Forwarded-For chain written by a trusted proxy in front - today it is dropped, so a service sees the load balancer`,
    },
  ],
  response: [
    {
      type: 'response-cache',
      doc: $localize`:@@planned_response_cache:Cache responses locally - held back: caching a personalised page serves it to the next visitor`,
    },
  ],
  // Nothing planned here: respond shipped, and it does more than the fixed
  // status and body this list once promised.
  terminal: [],
};
