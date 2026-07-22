// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

// Package collectors holds the real cloud-SDK enumerators that plug into the
// scan engine's CloudCollector seam. Keeping them out of internal/scanner keeps
// the core pipeline free of the heavy cloud SDK dependency trees.
package collectors

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/securityhub"
	shtypes "github.com/aws/aws-sdk-go-v2/service/securityhub/types"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// AWS is a real aws-sdk-go-v2 CloudCollector. It enumerates EC2 instances, S3
// buckets (with an encryption misconfig check) and Security Hub findings across
// the configured regions (or all enabled regions when none are given).
type AWS struct{}

// NewAWS returns the AWS cloud collector.
func NewAWS() scanner.CloudCollector { return AWS{} }

// bootstrapRegion is where region discovery and global (S3) calls originate.
const bootstrapRegion = "us-east-1"

func (AWS) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	base, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(bootstrapRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.Credentials["access_key_id"], cfg.Credentials["secret_access_key"], cfg.Credentials["session_token"],
		)),
	)
	if err != nil {
		errs <- fmt.Errorf("aws: load config: %w", err)
		return
	}

	regions := cfg.Regions
	if len(regions) == 0 {
		regions, err = discoverRegions(ctx, base)
		if err != nil {
			errs <- fmt.Errorf("aws: discover regions: %w", err)
			return
		}
	}

	// S3 is global — enumerate once from the bootstrap region.
	collectS3(ctx, base, assets, findings, errs)

	for _, region := range regions {
		if ctx.Err() != nil {
			errs <- ctx.Err()
			return
		}
		rcfg := base.Copy()
		rcfg.Region = region
		collectEC2(ctx, rcfg, region, assets, errs)
		collectSecurityHub(ctx, rcfg, region, findings, errs)
	}
}

func discoverRegions(ctx context.Context, cfg aws.Config) ([]string, error) {
	out, err := ec2.NewFromConfig(cfg).DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}
	regions := make([]string, 0, len(out.Regions))
	for _, r := range out.Regions {
		if r.RegionName != nil {
			regions = append(regions, *r.RegionName)
		}
	}
	return regions, nil
}

func collectEC2(ctx context.Context, cfg aws.Config, region string, assets chan<- scanner.AssetDiscovery, errs chan<- error) {
	pager := ec2.NewDescribeInstancesPaginator(ec2.NewFromConfig(cfg), &ec2.DescribeInstancesInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			errs <- fmt.Errorf("aws ec2 (%s): %w", region, err)
			return
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				assets <- ec2Asset(inst, region)
			}
		}
	}
}

func ec2Asset(inst ec2types.Instance, region string) scanner.AssetDiscovery {
	tags := map[string]string{}
	for _, t := range inst.Tags {
		if t.Key != nil && t.Value != nil {
			tags[*t.Key] = *t.Value
		}
	}
	name := tags["Name"]
	if name == "" {
		name = aws.ToString(inst.InstanceId)
	}
	platform := aws.ToString(inst.PlatformDetails)
	a := scanner.AssetDiscovery{
		ExternalID:  aws.ToString(inst.InstanceId),
		Name:        name,
		Type:        domain.AssetTypeVM,
		Environment: firstNonEmpty(tags["Environment"], tags["env"], tags["Env"]),
		CPE:         awsCPE(platform),
		Tags:        awsTags(tags, region),
		Location:    ptr(region),
		RawMetadata: map[string]any{"instance_type": string(inst.InstanceType), "platform": platform, "state": stateName(inst)},
	}
	if inst.PrivateIpAddress != nil {
		a.IP = inst.PrivateIpAddress
	} else if inst.PublicIpAddress != nil {
		a.IP = inst.PublicIpAddress
	}
	if inst.PrivateDnsName != nil && *inst.PrivateDnsName != "" {
		a.Hostname = inst.PrivateDnsName
	}
	if platform != "" {
		a.OS = ptr(platform)
	}
	return a
}

func collectS3(ctx context.Context, cfg aws.Config, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	client := s3.NewFromConfig(cfg)
	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		errs <- fmt.Errorf("aws s3: %w", err)
		return
	}
	for _, b := range out.Buckets {
		name := aws.ToString(b.Name)
		if name == "" {
			continue
		}
		assets <- scanner.AssetDiscovery{
			ExternalID: "arn:aws:s3:::" + name,
			Name:       name,
			Type:       domain.AssetTypeStorage,
			Tags:       []string{"s3"},
			CPE:        []string{"cpe:2.3:a:amazon:s3"},
		}
		// Misconfig: server-side encryption not configured.
		if _, encErr := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: b.Name}); encErr != nil {
			if strings.Contains(encErr.Error(), "ServerSideEncryptionConfigurationNotFound") {
				findings <- scanner.FindingDiscovery{
					Title:           "S3 bucket without default encryption",
					Description:     fmt.Sprintf("Bucket %q has no default server-side encryption configured.", name),
					Severity:        scanner.SeverityMedium,
					Evidence:        "GetBucketEncryption: ServerSideEncryptionConfigurationNotFound",
					RemediationHint: "Enable default SSE-S3 or SSE-KMS on the bucket.",
					Source:          "s3",
					AssetExternalID: "arn:aws:s3:::" + name,
					AffectedCPE:     []string{"cpe:2.3:a:amazon:s3"},
				}
			}
		}
	}
}

func collectSecurityHub(ctx context.Context, cfg aws.Config, region string, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	client := securityhub.NewFromConfig(cfg)
	filters := &shtypes.AwsSecurityFindingFilters{
		RecordState:   []shtypes.StringFilter{{Comparison: shtypes.StringFilterComparisonEquals, Value: aws.String("ACTIVE")}},
		SeverityLabel: severityFilters("CRITICAL", "HIGH", "MEDIUM"),
	}
	pager := securityhub.NewGetFindingsPaginator(client, &securityhub.GetFindingsInput{Filters: filters})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			// Security Hub may simply not be enabled in a region — that's not fatal.
			if strings.Contains(err.Error(), "not subscribed") || strings.Contains(err.Error(), "InvalidAccess") {
				return
			}
			errs <- fmt.Errorf("aws securityhub (%s): %w", region, err)
			return
		}
		for _, f := range page.Findings {
			findings <- securityHubFinding(f)
		}
	}
}

func securityHubFinding(f shtypes.AwsSecurityFinding) scanner.FindingDiscovery {
	sev := ""
	if f.Severity != nil {
		sev = string(f.Severity.Label)
	}
	out := scanner.FindingDiscovery{
		Title:           aws.ToString(f.Title),
		Description:     aws.ToString(f.Description),
		Severity:        sev,
		Evidence:        aws.ToString(f.ProductName),
		RemediationHint: remediationText(f),
		Source:          "security-hub",
	}
	if len(f.Resources) > 0 {
		out.AssetExternalID = aws.ToString(f.Resources[0].Id)
	}
	for _, v := range f.Vulnerabilities {
		if v.Id != nil && strings.HasPrefix(strings.ToUpper(*v.Id), "CVE-") {
			cve := strings.ToUpper(*v.Id)
			out.CVE = &cve
			break
		}
	}
	return out
}

// --- helpers ---------------------------------------------------------------

func severityFilters(labels ...string) []shtypes.StringFilter {
	fs := make([]shtypes.StringFilter, 0, len(labels))
	for _, l := range labels {
		fs = append(fs, shtypes.StringFilter{Comparison: shtypes.StringFilterComparisonEquals, Value: aws.String(l)})
	}
	return fs
}

func remediationText(f shtypes.AwsSecurityFinding) string {
	if f.Remediation != nil && f.Remediation.Recommendation != nil {
		return aws.ToString(f.Remediation.Recommendation.Text)
	}
	return ""
}

func stateName(inst ec2types.Instance) string {
	if inst.State != nil {
		return string(inst.State.Name)
	}
	return ""
}

func awsCPE(platformDetails string) []string {
	p := strings.ToLower(platformDetails)
	switch {
	case strings.Contains(p, "windows"):
		return []string{"cpe:2.3:o:microsoft:windows"}
	case strings.Contains(p, "red hat"), strings.Contains(p, "rhel"):
		return []string{"cpe:2.3:o:redhat:enterprise_linux"}
	case strings.Contains(p, "ubuntu"):
		return []string{"cpe:2.3:o:canonical:ubuntu_linux"}
	case strings.Contains(p, "linux"), strings.Contains(p, "unix"):
		return []string{"cpe:2.3:o:linux:linux_kernel"}
	default:
		return nil
	}
}

func awsTags(tags map[string]string, region string) []string {
	out := []string{"aws", region}
	for k, v := range tags {
		if k == "Name" {
			continue
		}
		out = append(out, fmt.Sprintf("%s:%s", k, v))
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func ptr[T any](v T) *T { return &v }
