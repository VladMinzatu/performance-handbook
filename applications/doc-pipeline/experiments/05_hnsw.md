# HNSW

Instead of our brute-force approach of doing exhaustive nearest neighbor search against all the vectors indexed so far, we will use a specialised index (see the GH repo for the fascinating details of how it works): 

```
go get github.com/coder/hnsw@main
```

After the change, let's try running the pipeline again, trying to generate 4000 docs per second:

![hnsw grafana app metrics](assets/hnsw_grafana_app.png)
![hnsw grafana internal metrics](assets/hnsw_grafana_internal.png)

So now we are able to get much better throughput (3000 docs/s out of the 4000 we are attempting to get). 
And it also looks like we are no longer CPU bound (though not by a huge margin).

