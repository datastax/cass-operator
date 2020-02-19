package oplabels

const (
	ManagedByLabel      = "app.kubernetes.io/managed-by"
	ManagedByLabelValue = "dse-operator"
)

func AddManagedByLabel(m map[string]string) {
	m[ManagedByLabel] = ManagedByLabelValue
}

func HasManagedByDseOperatorLabel(m map[string]string) bool {
	v, ok := m[ManagedByLabel]
	return ok && v == ManagedByLabelValue
}
