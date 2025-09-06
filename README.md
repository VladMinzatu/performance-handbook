# performance-handbook

In this repo I am writing a collection of small projects (mainly centered around system programming topics) and then doing performance and behaviour analysis on them while testing them in different conditions.

I will mainly be using Linux performance and debugging tools, eBPF tools like bpftrace and Go perf tools. And then I will document findings and comments. I'm running the tests on my Mac and, when necessary, in a Linux VM.

> ⚠️ **Note**: I am writing this code and running these experiments for **educational purposes**. As a general rule, I think one should always default to writing the *cleanest and most idiomatic* code that solves the problem, unless there is an actual performance or cost issue that needs to be solved or is worth solving. Moreover, performance analysis and issues debugging is an exercise that is influenced by the specifics of the application running under its target environment. So even if we compare different ways of achieving certain goals with some of the projects in here, we don't seek to draw the conclusion that one approach performs better than the other. These are exercises meant to illustrate the kinds of tools and techniques that are useful to perform such analysis.

The individual projects will be in subdirectories starting from here, and each will have its own README describing the project and test outcomes. For example:
- [wc-go](./wc-go)
- [log-aggregator](./log-aggregator)
- [reverse-proxy](./reverse-proxy)
- [fs-monitor](./fs-monitor)
- [sig-counter](./sig-counter)
- [db-query](./db-query)
