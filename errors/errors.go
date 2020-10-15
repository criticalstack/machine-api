package errors

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

func NewRequeueErrorf(d time.Duration, msg string, args ...interface{}) error {
	return errors.Wrapf(&RequeueAfterError{RequeueAfter: d}, msg, args...)
}

// HasRequeueAfterError represents that an actuator managed object should
// be requeued for further processing after the given RequeueAfter time has
// passed.
type HasRequeueAfterError interface {
	// GetRequeueAfter gets the duration to wait until the managed object is
	// requeued for further processing.
	GetRequeueAfter() time.Duration
}

// RequeueAfterError represents that an actuator managed object should be
// requeued for further processing after the given RequeueAfter time has
// passed.
type RequeueAfterError struct {
	RequeueAfter time.Duration
}

// Error implements the error interface
func (e *RequeueAfterError) Error() string {
	return fmt.Sprintf("requeue in %v", e.RequeueAfter)
}

// GetRequeueAfter gets the duration to wait until the managed object is
// requeued for further processing.
func (e *RequeueAfterError) GetRequeueAfter() time.Duration {
	return e.RequeueAfter
}

// IsRequeueAfter returns true if the error satisfies the interface HasRequeueAfterError.
func IsRequeueAfter(err error) bool {
	_, ok := errors.Cause(err).(HasRequeueAfterError)
	return ok
}

// Constants aren't automatically generated for unversioned packages.
// Instead share the same constant for all versioned packages
type MachineStatusError string

const (
	// Represents that the combination of configuration in the MachineSpec
	// is not supported by this cluster. This is not a transient error, but
	// indicates a state that must be fixed before progress can be made.
	//
	// Example: the ProviderSpec specifies an instance type that doesn't exist,
	InvalidConfigurationMachineError MachineStatusError = "InvalidConfiguration"

	// This indicates that the MachineSpec has been updated in a way that
	// is not supported for reconciliation on this cluster. The spec may be
	// completely valid from a configuration standpoint, but the controller
	// does not support changing the real world state to match the new
	// spec.
	//
	// Example: the responsible controller is not capable of changing the
	// container runtime from docker to rkt.
	UnsupportedChangeMachineError MachineStatusError = "UnsupportedChange"

	// This generally refers to exceeding one's quota in a cloud provider,
	// or running out of physical machines in an on-premise environment.
	InsufficientResourcesMachineError MachineStatusError = "InsufficientResources"

	// There was an error while trying to create a Node to match this
	// Machine. This may indicate a transient problem that will be fixed
	// automatically with time, such as a service outage, or a terminal
	// error during creation that doesn't match a more specific
	// MachineStatusError value.
	//
	// Example: timeout trying to connect to GCE.
	CreateMachineError MachineStatusError = "CreateError"

	// There was an error while trying to update a Node that this
	// Machine represents. This may indicate a transient problem that will be
	// fixed automatically with time, such as a service outage,
	//
	// Example: error updating load balancers
	UpdateMachineError MachineStatusError = "UpdateError"

	// An error was encountered while trying to delete the Node that this
	// Machine represents. This could be a transient or terminal error, but
	// will only be observable if the provider's Machine controller has
	// added a finalizer to the object to more gracefully handle deletions.
	//
	// Example: cannot resolve EC2 IP address.
	DeleteMachineError MachineStatusError = "DeleteError"

	// This error indicates that the machine did not join the cluster
	// as a new node within the expected timeframe after instance
	// creation at the provider succeeded
	//
	// Example use case: A controller that deletes Machines which do
	// not result in a Node joining the cluster within a given timeout
	// and that are managed by a MachineSet
	JoinClusterTimeoutMachineError = "JoinClusterTimeoutError"
)
