# performance-handbook

In this repo I am writing collection of small projects (mainly centered around system programming topics) and then doing performance and behaviour analysis on them while testing them in different conditions.

I will mainly be using Linux performance and debugging tools, eBPF tools like bpftrace and Go perf tools. And then I will document findings and comments. I'm running the tests on my Mac and, when necessary, in a Linux VM.

> ⚠️ **Note** I am writing this code and running these experiments for **educational purposes**. As a general rule, I think one should always default to writing the *cleanest and most idiomatic* code that solves the problem, unless there is an actual performance or cost issue that needs to be solved.

The individual projects will be in subdirectories starting from here, and each will have its own README describing the project and test outcomes. For example:
- [wc-go](./wc-go)
