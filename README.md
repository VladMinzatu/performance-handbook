# performance-handbook

In this repo I am writing a collection of small projects (mainly centered around system programming topics) and then doing performance and behaviour analysis on them while testing them in different conditions.

I will mainly be using Linux performance and debugging tools, eBPF tools like bpftrace and Go perf tools. And then I will document findings and comments. I'm running the tests on my Mac and, when necessary, in a Linux VM.

> ⚠️ **Note**: I am writing this code and running these experiments for **educational purposes**. As a general rule, I think one should always default to writing the _cleanest and most idiomatic_ code that solves the problem, unless there is an actual performance or cost issue that needs to be solved or is worth solving. Moreover, performance analysis and issues debugging is an exercise that is influenced by the specifics of the application running under its target environment. So even if we compare different ways of achieving certain goals with some of the projects in here, we don't seek to draw the conclusion that one approach performs better than the other in general. These are exercises meant to illustrate the kinds of tools and techniques that are useful to perform such analysis.

The [toolset](./toolset/) section of the repo covers the most relevant tools used for performance observability, analysis and debugging, ranging from the lowest level signals to business SLIs. It does not aim to substitute exhaustive documentation for each tool, but rather to paint the landscape and focus on where each tool fits in. So more emphasis is on the usability of the tool, like is it low-overhead enough for continuous production usage or should it only be used for offline analysis.

Then in the [applications](./applications/) subdirectory there are individual projects, each with its own README as the starting point for describing the project and test outcomes. For example:

- [doc-pipeline](./applications/doc-pipeline/)
- [wc-go](./applications/wc-go)
- [log-aggregator](./applications/log-aggregator)
- [reverse-proxy](./applications/reverse-proxy)
- [fs-monitor](./applications/fs-monitor)
- [sig-counter](./applications/sig-counter)
- [db-query](./applications/db-query)
