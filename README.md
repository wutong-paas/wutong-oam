# wutong-oam

## Instructions

The current repository serves as the toolkit repository for Wutong's OAM support.

## TODO

- [ ] Convert Wutong RAM to OAM.
- [ ] Convert OAM core workload to Wutong component.

## obstacles

* How to create ImagePullSecret?

> Trait? Rely on a trait controller that generates secret?

* How to deploy statefulset workload?

> Just use statefulset as the workload.

* How to make statefulset's `VolumeSource`?

> VolumeSource needs to match different resources to different cluster environments.

* Application cannot be installed multiple times for same namespace?

> May be specify the component name at installation time?

* Gets the injection variable from the dependent component.

> It is now possible to get the environment variables and set `DataOutput` one by one , inject them into each container of the downstream components one by one by `DataInput`.

* How to deal with storage dependencies between components?

> 