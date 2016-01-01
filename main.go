package main

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/jmcvetta/neoism"
)

var (
	testAddEc2Env = `
				WITH {vpcs} AS document
				UNWIND document.Vpcs as vpc
				MERGE (v:Vpc {VpcId:vpc.VpcId, State:vpc.State, CidrBlock:vpc.CidrBlock, IsDefault:vpc.IsDefault})

				WITH {subnets} AS document
				UNWIND document.Subnets as subnet
				MERGE (s:Subnet {SubnetId:subnet.SubnetId, State:subnet.State, CidrBlock:subnet.CidrBlock, State:subnet.State, DefaultForAz:subnet.DefaultForAz, AvailableIpAddressCount:subnet.AvailableIpAddressCount})
				MERGE (vpc:Vpc {VpcId:subnet.VpcId})
				MERGE (s)-[:IN_VPC]->(vpc)

				WITH {securitygroups} AS document
				UNWIND document.SecurityGroups as sg
				MERGE (s:SecurityGroup {GroupName:sg.GroupName, GroupId:sg.GroupId})
				FOREACH (ingressrule IN sg.IpPermissions |
					FOREACH(grouppair IN ingressrule.UserIdGroupPairs |
						MERGE (nsg:SecurityGroup {GroupId:grouppair.GroupId})
						MERGE (nsg)-[:HAS_INGRESS]->(s)
					)
				)
				FOREACH (egressrule IN sg.IpPermissionsEgress |
					FOREACH(grouppair IN egressrule.UserIdGroupPairs |
						MERGE (nsg:SecurityGroup {GroupId:grouppair.GroupId})
						MERGE (nsg)-[:HAS_EGRESS]->(s)
					)
				)
				MERGE (vpc:Vpc {VpcId:sg.VpcId})
				MERGE (s)-[:IN_VPC]->(vpc)

				WITH {instances} AS document
				UNWIND document.Reservations as reservation
				MERGE (r:Reservation {OwnerId:reservation.OwnerId, ReservationId:reservation.ReservationId})
				FOREACH (instance IN reservation.Instances |
					MERGE (m:Instance {InstanceId:instance.InstanceId, PublicIpAddress:instance.PublicIpAddress, PrivateIpAddress:instance.PrivateIpAddress})
					FOREACH (SecurityGroup IN instance.SecurityGroups |
						MERGE (sg:SecurityGroup {GroupName:SecurityGroup.GroupName, GroupId:SecurityGroup.GroupId})
						MERGE (m)-[:IN_SECURITY_GROUP]->(sg)
					)
					MERGE (s:Subnet {SubnetId:instance.SubnetId})
					MERGE (v:Vpc {VpcId:instance.VpcId})
					MERGE (m)-[:IN_SUBNET]->(s)
					//MERGE (m)-[:IN_VPC]->(v)
					//MERGE (m)-[:IN_RESERVATION]->(r)
				)`
)

func main() {
	// TODO(dan) configurable neo4j url
	db, err := neoism.Connect("http://localhost:7474/db/data")
	if err != nil {
		panic(err)
	}

	// TODO(dan) configurable region
	svc := ec2.New(session.New(), &aws.Config{Region: aws.String("us-west-2")})

	// get vpcs
	vpcs, err := svc.DescribeVpcs(nil)
	if err != nil {
		panic(err)
	}

	// get subnets
	subnets, err := svc.DescribeSubnets(nil)
	if err != nil {
		panic(err)
	}

	// get security groups
	securitygroups, err := svc.DescribeSecurityGroups(nil)
	if err != nil {
		panic(err)
	}

	// get reservations
	instances, err := svc.DescribeInstances(nil)
	if err != nil {
		panic(err)
	}

	// do it do it
	res := []struct{}{}
	cq := neoism.CypherQuery{
		Statement:  testAddEc2Env,
		Parameters: neoism.Props{"vpcs": vpcs, "subnets": subnets, "securitygroups": securitygroups, "instances": instances},
		Result:     &res,
	}

	err = db.Cypher(&cq)
	if err != nil {
		panic(err)
	}
	log.Print(res)

}
