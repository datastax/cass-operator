# Frequently Asked Questions

## My pods are stuck in a Pending state. Why won't they come up?

This one has a lot of potential answers...

- `kubectl describe pod` and check the message for why the current nodes aren't being used.
- Did you pin any `racks` to a `zone` in a different region, or one without any k8s workers?
