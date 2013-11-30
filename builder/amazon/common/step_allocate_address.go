package common

import (
	"fmt"
	"github.com/mitchellh/goamz/ec2"
	"github.com/mitchellh/multistep"
	"github.com/mitchellh/packer/packer"
	"log"
)

// StepAllocateAddress create a new EIP and associates it with an instance in the VPC.
//
// Produces:
//   addressAllocationId: string - The ID of the address allocation.
//   addressAssociationId: string - The ID of the address association.
//   publicIp: string - The Public IP of the instance.
type StepAllocateAddress struct {
	addressAllocationId  string
	addressAssociationId string
	publicIp             string
}

func (s *StepAllocateAddress) Run(state multistep.StateBag) multistep.StepAction {
	ec2conn := state.Get("ec2").(*ec2.EC2)
	instance := state.Get("instance").(*ec2.Instance)
	ui := state.Get("ui").(packer.Ui)

	if instance.VpcId == "" {
		// Ignore and continue.
		return multistep.ActionContinue
	}

	ui.Say("Allocating a new EIP...")
	allocateAddress := &ec2.AllocateAddress{
		Domain: "vpc",
	}
	log.Printf("Allocate args: %#v", allocateAddress)

	allocateAddressResp, err := ec2conn.AllocateAddress(allocateAddress)
	if err != nil {
		err := fmt.Errorf("Error allocating EIP: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	// Set the allocation ID so we remember to deallocate it later.
	s.addressAllocationId = allocateAddressResp.AllocationId
	log.Printf("Address Allocation ID: %s", s.addressAllocationId)

	// Set the public IP so we can connect to it later.
	s.publicIp = allocateAddressResp.PublicIp

	ui.Say(fmt.Sprintf("Associating new EIP: %s...", allocateAddressResp.PublicIp))
	// Associate the EIP with the VPC instance.
	associateAddress := &ec2.AssociateAddress{
		InstanceId:         instance.InstanceId,
		AllocationId:       allocateAddressResp.AllocationId,
		AllowReassociation: false,
	}
	associateAddressResp, err := ec2conn.AssociateAddress(associateAddress)

	if err != nil {
		// TODO: Deallocate address?
		err := fmt.Errorf("Error associating EIP: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	// Set the association ID so we remember to disassociate it later
	s.addressAssociationId = associateAddressResp.AssociationId
	log.Printf("Address Association ID: %s", s.addressAssociationId)

	state.Put("address_allocation_id", s.addressAllocationId)
	state.Put("public_ip", s.publicIp)
	state.Put("address_associate_id", s.addressAssociationId)
	return multistep.ActionContinue
}

func (s *StepAllocateAddress) Cleanup(state multistep.StateBag) {
	ec2conn := state.Get("ec2").(*ec2.EC2)
	ui := state.Get("ui").(packer.Ui)

	if s.addressAssociationId != "" {
		ui.Say("Disassociating the EIP...")
		_, err := ec2conn.DisassociateAddress(s.addressAssociationId)
		if err != nil {
			ui.Error(fmt.Sprintf("Error disassociating EIP: %s", err))
		}
	}

	if s.addressAllocationId != "" {
		ui.Say("Releasing the EIP...")
		_, err := ec2conn.ReleaseAddress(s.addressAllocationId)
		if err != nil {
			ui.Error(fmt.Sprintf("Error releasing EIP: %s", err))
		}
	}

	return
}
