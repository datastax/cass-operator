We've adjusted our procedures and rules for getting seed nodes at least a couple times, so I wanted to document the procedure in a way I'll remember it, and my approach for testing.

- Procedure

Pods should be labelled as a seed *after* they are ready to service traffic. Labelling them as a seed before has gotten us in trouble.

The *only* exception to the above is the state where there are no nodes that can serve traffic, like starting a fresh cluster OR unparking.

For me, it really helped to think about getting seeds into a good state as the task to do right before we start a DSE node.

Because it is best to follow the Kubernetes principle of resources being declarative, I coded the operator to label the lowest ordinal pod(s) in each statefulset as the seed nodes. This can create a situation where a seed pod dies, and the the operator labels a neighboring pod as a seed, brings the "original" seed back online, and then swaps the seed label to the pod it just started. I think this is all fine.

- Testing ideas

The basic (but comprehensive) scenario of...
* Bring up a three node / three rack cluster -> add three more nodes -> park the cluster -> unpark the cluster

In addition, I thought a good test was to...

* Bring up a nine node / three rack cluster -> `kubectl delete` the three seed nodes ->  watch the operator label three other nodes as seeds -> watch the operator bring the three missing nodes back online 

There's good output to understand what's going on when you start a long process and then watch output from...
```bash
kubectl -n dse-ns get pod -L com.datastax.dse.node.state,com.datastax.dse.seednode -w
```