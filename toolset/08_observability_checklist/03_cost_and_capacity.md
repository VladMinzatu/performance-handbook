# Cost and capacity

Cost and capacity are important engineering and business metrics that should not be ignored, though they should not be treated as SLIs/SLOs. This is because SLIs should be a quantitative measure of user-visible service behavior, so that violations can be associated with an error budget based on user impact and guide production acceptance decisions.

However, they should be measured, monitored, alerted on, included in dashboards, and subject to regular (e.g. weekly) review.

This might sound like the line is a bit blurry between monitored SLIs vs monitored metrics, but the idea is to not treat internal resource health the same as user-facing reliability outcomes. One category is clearly more important to prioritize without noise. Also, internal resource metrics can tend to be more volatile. That's why internal-resource SLOs are discouraged in general.

Examples:

- Unused slack capacity in cloud services or k8s resources is wasted money
- GPUs can be very expensive and their goodput should be optimised
