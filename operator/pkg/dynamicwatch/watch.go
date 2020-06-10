// Copyright DataStax, Inc.
// Please see the included license file for details.

package dynamicwatch

import (
	"reflect"
	"context"
	"encoding/json"
	"strings"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"github.com/datastax/cass-operator/operator/pkg/utils"
)

const (
	WatchedByAnnotation             = "cassandra.datastax.com/watched-by"
	WatchedLabel                    = "cassandra.datastax.com/watched"
)

type DynamicSecretWatches interface {
	UpdateSecretWatch(watcher types.NamespacedName, secrets []types.NamespacedName) error
	RemoveSecretWatch(watcher types.NamespacedName) error
	FindWatchers(meta metav1.Object, object runtime.Object) []types.NamespacedName
}

type DynamicSecretWatchesAnnotationImpl struct {
	Client runtimeClient.Client
	Ctx context.Context
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

func getWatcherNames(meta metav1.Object) []string {
	annotations := getAnnotationsOrEmptyMap(meta)
	content, ok := annotations[WatchedByAnnotation]

	if !ok {
		content = ""
	}
	
	data := []string{}
	if content != "" {
		err := json.Unmarshal([]byte(content), &data)
		if err != nil {
			// TODO: log a warning
			// As opposed to erroring out here, we'll just allow the
			// annotation to be replaced with a valid one
			data = []string{}
		}
	}

	return data
}

func hasWatchedLabel(meta metav1.Object) bool {
	labels := getLabelsOrEmptyMap(meta)
	value, ok := labels[WatchedLabel]
	return ok && "true" == value
}

func updateWatcherNames(meta metav1.Object, names []string) bool {
	originalWatchers := getWatcherNames(meta)
	originalHasWatchedLabel := hasWatchedLabel(meta)

	if len(names) == 0 {
		annotations := getAnnotationsOrEmptyMap(meta)
		delete(annotations, WatchedByAnnotation)
		meta.SetAnnotations(annotations)
		
		labels := getLabelsOrEmptyMap(meta)
		delete(labels, WatchedLabel)
		meta.SetLabels(labels)
	} else {
		bytes, err := json.Marshal(names)

		if err != nil {
			// TODO: Log an error
		} else {
			annotations := getAnnotationsOrEmptyMap(meta)
			annotations[WatchedByAnnotation] = string(bytes)
			meta.SetAnnotations(annotations)
			labels := getLabelsOrEmptyMap(meta)
			labels[WatchedLabel] = "true"
			meta.SetLabels(labels)
		}
	}
	newWatchers := getWatcherNames(meta)
	newHasWatchedLabel := hasWatchedLabel(meta)
	return !reflect.DeepEqual(originalWatchers, newWatchers) || originalHasWatchedLabel != newHasWatchedLabel
}

func removeWatcher(meta metav1.Object, watcher string) bool {
	watchers := getWatcherNames(meta)
	watchers = utils.RemoveValueFromStringArray(watchers, watcher)
	changedWatcherValues := updateWatcherNames(meta, watchers)

	return changedWatcherValues
}

func addWatcher(meta metav1.Object, watcher string) bool {
	watchers := getWatcherNames(meta)
	watchers = utils.AppendValuesToStringArrayIfNotPresent(watchers, watcher)
	return updateWatcherNames(meta, watchers)
}

func namespacedNameStringForSecret(secret *corev1.Secret) string {
	return namespacedNameToString(types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace,})
}

type toUpdate struct {
	secret *corev1.Secret
	patch client.Patch
}

func (impl *DynamicSecretWatchesAnnotationImpl) UpdateSecretWatch(watcher types.NamespacedName, watched []types.NamespacedName) error {
	// Since `watched` is a comprehensive list of what `watcher` is watching,
	// we can clean up any stale annotations on secrets no longer being watched
	// by `watcher`.
	watcherAsString := namespacedNameToString(watcher)
	secrets := corev1.SecretList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecretList",
			APIVersion: "v1",
		},
	}
	err := impl.Client.List(
		impl.Ctx,
		&secrets,
		client.MatchingLabels{"cassandra.datastax.com/watched": "true"})
	
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	secretsToUpdate := []toUpdate{}

	if err == nil {
		watchedAsStrings := namespacedNamesToStringArray(watched)
		for i := range secrets.Items {
			secret := &secrets.Items[i]
			patch := client.MergeFrom(secret.DeepCopy())
			namespacedNameAsString := namespacedNameStringForSecret(secret)
			if -1 == utils.FindValueIndexFromStringArray(watchedAsStrings, namespacedNameAsString) {
				// This is not a secret that `watcher` is watching. Make sure
				// `watcher` is not recorded as watching this secret in its
				// annotation.
				if removeWatcher(&secret.ObjectMeta, watcherAsString) {
					secretsToUpdate = append(secretsToUpdate, toUpdate{secret: secret, patch: patch,})
				}
			}

		}
	}

	// Now we need to add `watcher` to the relevant secrets
	for _, name := range watched {
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
			},
		}
	
		err := impl.Client.Get(
			impl.Ctx,
			name,
			secret)
	
		if err != nil {
			if errors.IsNotFound(err) {
				// we are attempting to watch a secret that does not exist...
				// TODO: Log warning
			} else {
				return err
			}
		}

		patch := client.MergeFrom(secret.DeepCopy())

		if addWatcher(&secret.ObjectMeta, watcherAsString) {
			secretsToUpdate = append(secretsToUpdate, toUpdate{secret: secret, patch: patch,})
		}
	}

	// persist the watch state
	errors := []error{}
	for _, update := range secretsToUpdate {
		err := impl.Client.Patch(impl.Ctx, update.secret, update.patch)
		if err != nil {
			// make a best effort to update as many secrets as possible
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return errors[0]
	} else {
		return nil
	}
}

func (impl *DynamicSecretWatchesAnnotationImpl) RemoveSecretWatch(watcher types.NamespacedName) error {
	return impl.UpdateSecretWatch(watcher, []types.NamespacedName{})
}

func (impl *DynamicSecretWatchesAnnotationImpl) FindWatchers(meta metav1.Object, object runtime.Object) []types.NamespacedName {
	watchersAsStrings := getWatcherNames(meta)
	return namespacedNamesFromStringArray(watchersAsStrings)
}
