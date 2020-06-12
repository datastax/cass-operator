// Copyright DataStax, Inc.
// Please see the included license file for details.

package dynamicwatch

import (
	"reflect"
	"context"
	"encoding/json"
	"strings"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"github.com/datastax/cass-operator/operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	WatchedByAnnotation = "cassandra.datastax.com/watched-by"
	WatchedLabel        = "cassandra.datastax.com/watched"
)

type DynamicWatches interface {
	UpdateWatch(watcher types.NamespacedName, watched []types.NamespacedName) error
	RemoveWatcher(watcher types.NamespacedName) error
	FindWatchers(meta metav1.Object, object runtime.Object) []types.NamespacedName
}

type DynamicWatchesAnnotationImpl struct {
	Client          runtimeClient.Client
	Ctx             context.Context
	WatchedType     metav1.TypeMeta
	WatchedListType metav1.TypeMeta
	Logger          logr.Logger
}

func NewDynamicSecretWatches(client client.Client) DynamicWatches {
	impl := &DynamicWatchesAnnotationImpl{
		Client: client,
		Ctx: context.Background(),
		WatchedType: metav1.TypeMeta{
			APIVersion: "v1",
			Kind: "Secret",
		},
		WatchedListType: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "SecretList",
		},
		Logger: logf.Log.WithName("dynamicwatches"),
	}
	return impl
}

//
// Utility functions
//

func namespacedNameString(meta metav1.Object) string {
	return namespacedNameToString(types.NamespacedName{Name: meta.GetName(), Namespace: meta.GetNamespace(),})
}

func namespacedNameFromString(s string) types.NamespacedName {
	comps := strings.Split(s, "/")
	name := comps[len(comps)-1]
	namespace := strings.TrimSuffix(s, name)
	namespace = strings.TrimRight(namespace, "/")

	return types.NamespacedName{Name: name, Namespace: namespace,}
}

// There does not appear to be an explicit guarantee that 
// NamespacedName.String() will produced output in a given format. It's quite
// unlikely it will ever change, but to be safe, we implement our own 
// serialization to a string.
func namespacedNameToString(n types.NamespacedName) string {
	return fmt.Sprintf("%s/%s", n.Namespace, n.Name)
}

func namespacedNamesToStringArray(names []types.NamespacedName) []string {
	nameStrings := []string{}
	for _, name := range names {
		nameStrings = append(nameStrings, namespacedNameToString(name))
	}
	return nameStrings
}

func namespacedNamesFromStringArray(names []string) []types.NamespacedName {
	namespacedNames := []types.NamespacedName{}
	for _, name := range names {
		namespacedNames = append(namespacedNames, namespacedNameFromString(name))
	}
	return namespacedNames
}

func getAnnotationsOrEmptyMap(meta metav1.Object) map[string]string {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	return annotations
}

func getLabelsOrEmptyMap(meta metav1.Object) map[string]string {
	labels := meta.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	return labels
}

//
// Functions for loading and saving watchers in an annotation
// 

func (impl *DynamicWatchesAnnotationImpl) getWatcherNames(watched metav1.Object) []string {
	annotations := getAnnotationsOrEmptyMap(watched)
	content, ok := annotations[WatchedByAnnotation]

	if !ok {
		content = ""
	}
	
	data := []string{}
	if content != "" {
		err := json.Unmarshal([]byte(content), &data)
		if err != nil {
			impl.Logger.Error(err, "Failed to parse watchers data", 
				"watched", namespacedNameString(watched))

			// As opposed to erroring out here, we'll just allow the
			// annotation to be replaced with a valid one
			data = []string{}
		}
	}

	return data
}

func (impl *DynamicWatchesAnnotationImpl) updateWatcherNames(watched metav1.Object, watcherNames []string) bool {
	originalWatchers := impl.getWatcherNames(watched)
	originalHasWatchedLabel := hasWatchedLabel(watched)

	if len(watcherNames) == 0 {
		annotations := getAnnotationsOrEmptyMap(watched)
		delete(annotations, WatchedByAnnotation)
		watched.SetAnnotations(annotations)

		labels := getLabelsOrEmptyMap(watched)
		delete(labels, WatchedLabel)
		watched.SetLabels(labels)
	} else {
		bytes, err := json.Marshal(watcherNames)

		if err != nil {
			impl.Logger.Error(err, "Failed to updated watchers on watched resource", "watched", namespacedNameString(watched))
		} else {
			annotations := getAnnotationsOrEmptyMap(watched)
			annotations[WatchedByAnnotation] = string(bytes)
			watched.SetAnnotations(annotations)
			labels := getLabelsOrEmptyMap(watched)
			labels[WatchedLabel] = "true"
			watched.SetLabels(labels)
		}
	}
	newWatchers := impl.getWatcherNames(watched)
	newHasWatchedLabel := hasWatchedLabel(watched)
	return !reflect.DeepEqual(originalWatchers, newWatchers) || originalHasWatchedLabel != newHasWatchedLabel
}

//
// Functions for manipulating watchers in annotation
//

func (impl *DynamicWatchesAnnotationImpl) removeWatcher(watched metav1.Object, watcher string) bool {
	watchers := impl.getWatcherNames(watched)
	watchers = utils.RemoveValueFromStringArray(watchers, watcher)
	changedWatcherValues := impl.updateWatcherNames(watched, watchers)

	return changedWatcherValues
}

func (impl *DynamicWatchesAnnotationImpl) addWatcher(watched metav1.Object, watcher string) bool {
	watchers := impl.getWatcherNames(watched)
	watchers = utils.AppendValuesToStringArrayIfNotPresent(watchers, watcher)
	return impl.updateWatcherNames(watched, watchers)
}

func hasWatchedLabel(meta metav1.Object) bool {
	labels := getLabelsOrEmptyMap(meta)
	value, ok := labels[WatchedLabel]
	return ok && "true" == value
}

//
// DAO functions
//

func (impl *DynamicWatchesAnnotationImpl) listAllWatched(namespace string) ([]unstructured.Unstructured, error) {
	watchedList := &unstructured.UnstructuredList{}
	watchedList.SetKind(impl.WatchedListType.Kind)
	watchedList.SetAPIVersion(impl.WatchedListType.APIVersion)

	selector := map[string]string{WatchedLabel: "true",}

	listOptions := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(selector),
		Namespace: namespace,
	}

	err := impl.Client.List(
		impl.Ctx,
		watchedList,
		listOptions)
	
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err	
		}
	}

	return watchedList.Items, nil
}

func (impl *DynamicWatchesAnnotationImpl) getWatched(watched types.NamespacedName) (*unstructured.Unstructured, error) {
	watchedItem := &unstructured.Unstructured{}
	watchedItem.SetKind(impl.WatchedType.Kind)
	watchedItem.SetAPIVersion(impl.WatchedType.APIVersion)
	watchedItem.SetName(watched.Name)
	watchedItem.SetNamespace(watched.Namespace)

	err := impl.Client.Get(
		impl.Ctx,
		watched,
		watchedItem)

	if err != nil {
		return nil, err
	}

	return watchedItem, nil
}

//
// Core implementation functions of DynamicWatcher interface
//

type toUpdate struct {
	watchedItem *unstructured.Unstructured
	patch       client.Patch
}

func (impl *DynamicWatchesAnnotationImpl) UpdateWatch(watcher types.NamespacedName, watched []types.NamespacedName) error {
	// Since `watched` is a comprehensive list of what `watcher` is watching,
	// we can clean up any stale annotations on resources no longer being watched
	// by `watcher`.
	watcherAsString := namespacedNameToString(watcher)

	items, err := impl.listAllWatched(watcher.Namespace)
	if err != nil {
		return err
	}

	itemsToUpdate := []toUpdate{}

	if err == nil {
		watchedAsStrings := namespacedNamesToStringArray(watched)
		for i := range items {
			watchedItem := &items[i]
			patch := client.MergeFrom(watchedItem.DeepCopy())
			namespacedNameAsString := namespacedNameString(watchedItem)
			if -1 == utils.IndexOfString(watchedAsStrings, namespacedNameAsString) {
				// This is not a resource that `watcher` is watching. Make sure
				// `watcher` is not recorded as watching this resource in its
				// annotation.
				if impl.removeWatcher(watchedItem, watcherAsString) {
					itemsToUpdate = append(itemsToUpdate, toUpdate{watchedItem: watchedItem, patch: patch,})
				}
			}

		}
	}

	// Now we need to add `watcher` to the relevant resource
	for _, name := range watched {
		watchedItem, err := impl.getWatched(name)
	
		if err != nil {
			if errors.IsNotFound(err) {
				// we are attempting to watch a resource that does not exist...
				impl.Logger.Error(err, "Watched resource not found", "watched", namespacedNameString(watchedItem))
				continue
			} else {
				return err
			}
		}

		patch := client.MergeFrom(watchedItem.DeepCopy())

		if impl.addWatcher(watchedItem, watcherAsString) {
			itemsToUpdate = append(itemsToUpdate, toUpdate{watchedItem: watchedItem, patch: patch,})
		}
	}

	// Persist the watch state
	errors := []error{}
	for _, update := range itemsToUpdate {
		err := impl.Client.Patch(impl.Ctx, update.watchedItem, update.patch)
		if err != nil {
			// make a best effort to update as many resources as possible
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return errors[0]
	} else {
		return nil
	}
}

func (impl *DynamicWatchesAnnotationImpl) RemoveWatcher(watcher types.NamespacedName) error {
	return impl.UpdateWatch(watcher, []types.NamespacedName{})
}

func (impl *DynamicWatchesAnnotationImpl) FindWatchers(watchedMeta metav1.Object, watchedObject runtime.Object) []types.NamespacedName {
	watchersAsStrings := impl.getWatcherNames(watchedMeta)
	return namespacedNamesFromStringArray(watchersAsStrings)
}
