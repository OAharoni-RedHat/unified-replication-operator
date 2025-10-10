/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  This is scaffolding for you to own.
// NOTE: json tags are required.  Any new fields you add must have json:"-" or json:"foo" tags for the fields to be serialized.

// ReplicationState defines the desired replication state (role-based)
// +kubebuilder:validation:Enum=source;replica;promoting;demoting;syncing;failed
type ReplicationState string

const (
	// ReplicationStateSource indicates the volume is the primary source
	ReplicationStateSource ReplicationState = "source"
	// ReplicationStateReplica indicates the volume is a replica
	ReplicationStateReplica ReplicationState = "replica"
	// ReplicationStatePromoting indicates the volume is being promoted from replica to source
	ReplicationStatePromoting ReplicationState = "promoting"
	// ReplicationStateDemoting indicates the volume is being demoted from source to replica
	ReplicationStateDemoting ReplicationState = "demoting"
	// ReplicationStateSyncing indicates the volume is synchronizing
	ReplicationStateSyncing ReplicationState = "syncing"
	// ReplicationStateFailed indicates the replication has failed
	ReplicationStateFailed ReplicationState = "failed"
)

// ReplicationMode defines the replication consistency mode
// +kubebuilder:validation:Enum=synchronous;asynchronous;eventual
type ReplicationMode string

const (
	// ReplicationModeSynchronous provides synchronous replication
	ReplicationModeSynchronous ReplicationMode = "synchronous"
	// ReplicationModeAsynchronous provides asynchronous replication
	ReplicationModeAsynchronous ReplicationMode = "asynchronous"
	// ReplicationModeEventual provides eventual consistency replication
	ReplicationModeEventual ReplicationMode = "eventual"
)

// ScheduleMode defines the replication scheduling mode
// +kubebuilder:validation:Enum=continuous;interval;manual
type ScheduleMode string

const (
	// ScheduleModeContinuous provides continuous replication
	ScheduleModeContinuous ScheduleMode = "continuous"
	// ScheduleModeInterval provides interval-based replication
	ScheduleModeInterval ScheduleMode = "interval"
	// ScheduleModeManual provides manual replication triggers
	ScheduleModeManual ScheduleMode = "manual"
)

// Endpoint defines a replication endpoint with cluster, region, and storage information
type Endpoint struct {
	// Cluster identifier for the Kubernetes cluster
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Cluster string `json:"cluster" yaml:"cluster"`

	// Region identifier for geographic location
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Region string `json:"region" yaml:"region"`

	// StorageClass name for the storage system
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	StorageClass string `json:"storageClass" yaml:"storageClass"`
}

// VolumeSource defines the source volume information
type VolumeSource struct {
	// PVC name in the source cluster
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PvcName string `json:"pvcName" yaml:"pvcName"`

	// Namespace containing the PVC
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace" yaml:"namespace"`
}

// VolumeDestination defines the destination volume information
type VolumeDestination struct {
	// VolumeHandle is the backend-specific volume identifier
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	VolumeHandle string `json:"volumeHandle" yaml:"volumeHandle"`

	// Namespace for the destination volume
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace" yaml:"namespace"`
}

// VolumeMapping defines the source to destination volume mapping
type VolumeMapping struct {
	// Source volume information
	// +kubebuilder:validation:Required
	Source VolumeSource `json:"source" yaml:"source"`

	// Destination volume information
	// +kubebuilder:validation:Required
	Destination VolumeDestination `json:"destination" yaml:"destination"`
}

// Schedule defines replication scheduling configuration
type Schedule struct {
	// RPO (Recovery Point Objective) - maximum acceptable data loss duration
	// +kubebuilder:validation:Pattern=`^[0-9]+(s|m|h|d)$`
	// +optional
	Rpo string `json:"rpo,omitempty" yaml:"rpo,omitempty"`

	// RTO (Recovery Time Objective) - maximum acceptable recovery time
	// +kubebuilder:validation:Pattern=`^[0-9]+(s|m|h|d)$`
	// +optional
	Rto string `json:"rto,omitempty" yaml:"rto,omitempty"`

	// Mode defines the scheduling approach
	// +kubebuilder:validation:Required
	Mode ScheduleMode `json:"mode" yaml:"mode"`
}

// CephExtensions defines Ceph-specific configuration
type CephExtensions struct {
	// MirroringMode specifies the RBD mirroring mode
	// +kubebuilder:validation:Enum=journal;snapshot
	// +optional
	MirroringMode *string `json:"mirroringMode,omitempty" yaml:"mirroringMode,omitempty"`
}

// TridentExtensions defines Trident-specific configuration
// Currently empty but reserved for future Trident-specific settings
type TridentExtensions struct {
}

// PowerStoreExtensions defines PowerStore-specific configuration
// Currently empty but reserved for future PowerStore-specific settings
type PowerStoreExtensions struct {
}

// Extensions defines vendor-specific extension configurations
type Extensions struct {
	// Ceph-specific extensions
	// +optional
	Ceph *CephExtensions `json:"ceph,omitempty" yaml:"ceph,omitempty"`

	// Trident-specific extensions
	// +optional
	Trident *TridentExtensions `json:"trident,omitempty" yaml:"trident,omitempty"`

	// PowerStore-specific extensions
	// +optional
	Powerstore *PowerStoreExtensions `json:"powerstore,omitempty" yaml:"powerstore,omitempty"`
}

// UnifiedVolumeReplicationSpec defines the desired state of UnifiedVolumeReplication
type UnifiedVolumeReplicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SourceEndpoint defines the source replication endpoint
	// +kubebuilder:validation:Required
	SourceEndpoint Endpoint `json:"sourceEndpoint" yaml:"sourceEndpoint"`

	// DestinationEndpoint defines the destination replication endpoint
	// +kubebuilder:validation:Required
	DestinationEndpoint Endpoint `json:"destinationEndpoint" yaml:"destinationEndpoint"`

	// VolumeMapping defines the source to destination volume mapping
	// +kubebuilder:validation:Required
	VolumeMapping VolumeMapping `json:"volumeMapping" yaml:"volumeMapping"`

	// ReplicationState defines the desired replication state
	// +kubebuilder:validation:Required
	ReplicationState ReplicationState `json:"replicationState" yaml:"replicationState"`

	// ReplicationMode defines the replication consistency mode
	// +kubebuilder:validation:Required
	ReplicationMode ReplicationMode `json:"replicationMode" yaml:"replicationMode"`

	// Schedule defines the replication scheduling configuration
	// +kubebuilder:validation:Required
	Schedule Schedule `json:"schedule" yaml:"schedule"`

	// Extensions for vendor-specific configurations
	// +optional
	Extensions *Extensions `json:"extensions,omitempty" yaml:"extensions,omitempty"`
}

// UnifiedVolumeReplicationStatus defines the observed state of UnifiedVolumeReplication
type UnifiedVolumeReplicationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions represent the latest available observations of the replication's current state
	// +optional
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DiscoveredBackends lists the storage backends discovered in the cluster
	// +optional
	DiscoveredBackends []BackendInfo `json:"discoveredBackends,omitempty"`
}

// BackendInfo provides information about discovered storage backends
type BackendInfo struct {
	// Name of the backend
	Name string `json:"name"`

	// Type of the backend
	// +kubebuilder:validation:Enum=ceph-csi;trident;powerstore
	Type string `json:"type"`

	// Available indicates if the backend is available
	Available bool `json:"available"`

	// Capabilities lists the backend's capabilities
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,shortName=uvr;unifiedvr
//+kubebuilder:printcolumn:name="State",type="string",JSONPath=".spec.replicationState"
//+kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.replicationMode"
//+kubebuilder:printcolumn:name="Source",type="string",JSONPath=".spec.volumeMapping.source.pvcName"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// UnifiedVolumeReplication is the Schema for the unifiedvolumereplications API
type UnifiedVolumeReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnifiedVolumeReplicationSpec   `json:"spec,omitempty"`
	Status UnifiedVolumeReplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UnifiedVolumeReplicationList contains a list of UnifiedVolumeReplication
type UnifiedVolumeReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UnifiedVolumeReplication `json:"items"`
}

// Validation methods and helpers

var (
	// timePatternRegex validates time duration patterns like "5m", "1h", "30s", "1d"
	timePatternRegex = regexp.MustCompile(`^[0-9]+(s|m|h|d)$`)
)

// ValidateSpec performs comprehensive validation of the UnifiedVolumeReplication spec
func (uvr *UnifiedVolumeReplication) ValidateSpec() error {
	if err := uvr.validateEndpoints(); err != nil {
		return err
	}

	if err := uvr.validateVolumeMapping(); err != nil {
		return err
	}

	if err := uvr.validateSchedule(); err != nil {
		return err
	}

	if err := uvr.validateExtensions(); err != nil {
		return err
	}

	return nil
}

// validateEndpoints ensures source and destination endpoints are different and valid
func (uvr *UnifiedVolumeReplication) validateEndpoints() error {
	src := uvr.Spec.SourceEndpoint
	dest := uvr.Spec.DestinationEndpoint

	// Cross-field validation: source != destination
	if src.Cluster == dest.Cluster && src.Region == dest.Region && src.StorageClass == dest.StorageClass {
		return fmt.Errorf("source and destination endpoints cannot be identical (cluster: %s, region: %s, storageClass: %s)",
			src.Cluster, src.Region, src.StorageClass)
	}

	// Validate endpoint field formats
	if err := validateEndpointFields(src, "source"); err != nil {
		return err
	}

	if err := validateEndpointFields(dest, "destination"); err != nil {
		return err
	}

	return nil
}

// validateEndpointFields validates individual endpoint field values
func validateEndpointFields(endpoint Endpoint, endpointType string) error {
	if strings.TrimSpace(endpoint.Cluster) == "" {
		return fmt.Errorf("%s endpoint cluster cannot be empty", endpointType)
	}

	if strings.TrimSpace(endpoint.Region) == "" {
		return fmt.Errorf("%s endpoint region cannot be empty", endpointType)
	}

	if strings.TrimSpace(endpoint.StorageClass) == "" {
		return fmt.Errorf("%s endpoint storageClass cannot be empty", endpointType)
	}

	// Validate naming conventions
	if !isValidKubernetesName(endpoint.Cluster) {
		return fmt.Errorf("%s endpoint cluster name '%s' is not a valid Kubernetes name", endpointType, endpoint.Cluster)
	}

	if !isValidKubernetesName(endpoint.StorageClass) {
		return fmt.Errorf("%s endpoint storageClass name '%s' is not a valid Kubernetes name", endpointType, endpoint.StorageClass)
	}

	return nil
}

// validateVolumeMapping validates the volume mapping configuration
func (uvr *UnifiedVolumeReplication) validateVolumeMapping() error {
	mapping := uvr.Spec.VolumeMapping

	// Validate source
	if strings.TrimSpace(mapping.Source.PvcName) == "" {
		return fmt.Errorf("volume mapping source pvcName cannot be empty")
	}

	if strings.TrimSpace(mapping.Source.Namespace) == "" {
		return fmt.Errorf("volume mapping source namespace cannot be empty")
	}

	// Validate destination
	if strings.TrimSpace(mapping.Destination.VolumeHandle) == "" {
		return fmt.Errorf("volume mapping destination volumeHandle cannot be empty")
	}

	if strings.TrimSpace(mapping.Destination.Namespace) == "" {
		return fmt.Errorf("volume mapping destination namespace cannot be empty")
	}

	// Validate Kubernetes naming conventions
	if !isValidKubernetesName(mapping.Source.PvcName) {
		return fmt.Errorf("volume mapping source pvcName '%s' is not a valid Kubernetes name", mapping.Source.PvcName)
	}

	if !isValidKubernetesName(mapping.Source.Namespace) {
		return fmt.Errorf("volume mapping source namespace '%s' is not a valid Kubernetes name", mapping.Source.Namespace)
	}

	if !isValidKubernetesName(mapping.Destination.Namespace) {
		return fmt.Errorf("volume mapping destination namespace '%s' is not a valid Kubernetes name", mapping.Destination.Namespace)
	}

	return nil
}

// validateSchedule validates the schedule configuration
func (uvr *UnifiedVolumeReplication) validateSchedule() error {
	schedule := uvr.Spec.Schedule

	// Validate RPO pattern if provided
	if schedule.Rpo != "" && !timePatternRegex.MatchString(schedule.Rpo) {
		return fmt.Errorf("schedule RPO '%s' does not match required pattern (e.g., '5m', '1h', '30s', '1d')", schedule.Rpo)
	}

	// Validate RTO pattern if provided
	if schedule.Rto != "" && !timePatternRegex.MatchString(schedule.Rto) {
		return fmt.Errorf("schedule RTO '%s' does not match required pattern (e.g., '5m', '1h', '30s', '1d')", schedule.Rto)
	}

	// Mode-specific validation
	switch schedule.Mode {
	case ScheduleModeInterval:
		if schedule.Rpo == "" {
			return fmt.Errorf("schedule RPO is required when mode is 'interval'")
		}
	case ScheduleModeContinuous:
		// For continuous mode, RPO/RTO are optional as they represent target objectives
	case ScheduleModeManual:
		// For manual mode, RPO/RTO are optional as they represent recovery targets
	default:
		return fmt.Errorf("invalid schedule mode '%s', must be one of: continuous, interval, manual", schedule.Mode)
	}

	return nil
}

// validateExtensions validates vendor-specific extensions
func (uvr *UnifiedVolumeReplication) validateExtensions() error {
	if uvr.Spec.Extensions == nil {
		return nil // Extensions are optional
	}

	extensions := uvr.Spec.Extensions

	// Validate Ceph extensions
	if extensions.Ceph != nil {
		if err := validateCephExtensions(extensions.Ceph); err != nil {
			return fmt.Errorf("ceph extensions validation failed: %w", err)
		}
	}

	// Validate Trident extensions
	if extensions.Trident != nil {
		if err := validateTridentExtensions(extensions.Trident); err != nil {
			return fmt.Errorf("trident extensions validation failed: %w", err)
		}
	}

	// Validate PowerStore extensions
	if extensions.Powerstore != nil {
		if err := validatePowerStoreExtensions(extensions.Powerstore); err != nil {
			return fmt.Errorf("powerstore extensions validation failed: %w", err)
		}
	}

	return nil
}

// validateCephExtensions validates Ceph-specific configuration
func validateCephExtensions(ceph *CephExtensions) error {
	if ceph.MirroringMode != nil {
		validModes := []string{"journal", "snapshot"}
		if !contains(validModes, *ceph.MirroringMode) {
			return fmt.Errorf("invalid mirroring mode '%s', must be one of: %s", *ceph.MirroringMode, strings.Join(validModes, ", "))
		}
	}

	return nil
}

// validateTridentExtensions validates Trident-specific configuration
func validateTridentExtensions(trident *TridentExtensions) error {
	// No validation needed - struct is empty but reserved for future use
	return nil
}

// validatePowerStoreExtensions validates PowerStore-specific configuration
func validatePowerStoreExtensions(powerstore *PowerStoreExtensions) error {
	// No validation needed - struct is empty but reserved for future use
	return nil
}

// Helper functions

// isValidKubernetesName checks if a string is a valid Kubernetes resource name
func isValidKubernetesName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	// Kubernetes resource names must be lowercase alphanumeric, with dashes and dots allowed
	// but not at the beginning or end
	kubernetesNameRegex := regexp.MustCompile(`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`)
	return kubernetesNameRegex.MatchString(name)
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
