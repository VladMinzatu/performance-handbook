# Business SLIs

SLIs that correspond to business operations that span multiple applications (department or org level). They follow the regular application level SLO rules in terms of their definition and reporting.

Each should have an associated impact (e.g. updated regularly, or queryable in real time in a dashboard based on analytcs data).

Same as the set of an application's SLOs should be minimal while capturing the operational health of the application, the business SLIs should capture the operational health of the business flows that an org is responsible for, while having clear associated impact.

The health check for this is if incidents translate to business SLIs being impacted, and in turn, incident impact is reasonably easy to estimate. Incidents should implicitly trigger a the consideration of action items to improve the business SLI definitions.

A dashboard tracks the error budget over 7 and 28 days, reviewed weekly at org level.
