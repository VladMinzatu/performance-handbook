# Operational Dashboards

These are per-application-dashboards or systems that capture observability at the application level, meant for team (e.g. weekly) review.
(ideally the dashboard captures everything that's relevant so the review can consist of just going over all applications' dashboards)

## Application dashboard

An application

### SLO

7 and 28 day error budget (and burn dnwn) gauges/graphs reporting. Ideally for each, SLO there is burn-rate alerting enabled.

The usual guidelines for SLO definition apply:

- minimal set of SLOs that cover the user-facing reliability of the system
- defined as ratio of good events over total; trackable in real time
- define meaningful SLIs that capture business-logic-related subtleties of user experiences, for different use cases or customer groups
- etc.

### Deep Dive section

Graphs for:

- the 4 golden signals.
- downstream calls: rps, latency percentiles, error rates, circuit-breaker metrics.

### Cost and capacity

Plug into the platform metrics that should be there (see next section) and include the relevant ones for this application as a section in the dashboard.

### Alerts and incidents

Especially relevant for the weekly reviews - it's handy if a numeric widget with a link is included here. (again, provided as platform to just plug in)
