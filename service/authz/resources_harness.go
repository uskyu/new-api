package authz

const (
	ResourceHarness            = "harness"
	ResourceIntelligentRouting = "intelligent_routing"
)

var (
	HarnessRead             = Permission{Resource: ResourceHarness, Action: ActionRead}
	IntelligentRoutingRead  = Permission{Resource: ResourceIntelligentRouting, Action: ActionRead}
	IntelligentRoutingWrite = Permission{Resource: ResourceIntelligentRouting, Action: ActionWrite}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceHarness,
		LabelKey: "Harness Integration",
		Actions: []ActionDefinition{{
			Action:         ActionRead,
			LabelKey:       "Read Harness model catalog",
			DescriptionKey: "Read the authorized model, resource pool, capability, and pricing catalog for Harness.",
			DefaultRoles:   []string{BuiltInRoleAdmin},
		}},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceIntelligentRouting,
		LabelKey: "Intelligent Routing",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read intelligent routing", DescriptionKey: "View global route priority and resource pool settings.", DefaultRoles: []string{BuiltInRoleAdmin}},
			{Action: ActionWrite, LabelKey: "Edit intelligent routing", DescriptionKey: "Configure global route priority, resource pools, and token access.", DefaultRoles: []string{BuiltInRoleAdmin}},
		},
	})
}
