# Cost and capacity

This refers to cluster and node usage and cost; network; container resource usage metrics (CPU, memory, io) and slack; as well as managed service capacity, usage and cost, etc.

Cost and capacity are important engineering and business metrics that should not be ignored, though they should not be treated as SLIs/SLOs. This is because SLIs should be a quantitative measure of user-visible service behavior, so that violations can be associated with an error budget based on user impact and guide production acceptance decisions.

However, they should be measured, monitored, alerted on, included in dashboards, and subject to regular (e.g. weekly) review.

Ideally, this kind of monitoring should be provided as "batteries included" at the platform level, with minimal effort for teams to capture them in dashboards and define alerts when they use new services or have new applications.

Visibility should be both for product teams, as well as for org-wide reports and they should be reviewed in both formats.

This might sound like the line is a bit blurry between monitored SLIs vs monitored metrics, but the idea is to not treat internal resource health the same as user-facing reliability outcomes. One category is clearly more important to prioritize without noise. Also, internal resource metrics can tend to be more volatile. That's why internal-resource SLOs are discouraged in general.

Examples:

- Unused slack capacity in cloud services or k8s resources is wasted money
- GPUs can be very expensive and their goodput should be optimised
