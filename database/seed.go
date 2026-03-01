package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	authDomain "parkieee/internal/modules/auth"
	"parkieee/pkg/types"
)

// Seed inserts default lookup data that must exist for the app to function.
// It is idempotent — safe to run multiple times (uses ON CONFLICT DO NOTHING).
func Seed(db *gorm.DB) error {
	if err := seedRoles(db); err != nil {
		return fmt.Errorf("seed roles: %w", err)
	}
	if err := seedPermissions(db); err != nil {
		return fmt.Errorf("seed permissions: %w", err)
	}
	if err := seedRolePermissions(db); err != nil {
		return fmt.Errorf("seed role_permissions: %w", err)
	}
	if err := seedAdminUser(db); err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	return nil
}

func seedRoles(db *gorm.DB) error {
	roles := []authDomain.Role{
		{ID: roleID("operator"), Name: string(types.RoleOperator), Description: "Gate operator — handles entry/exit and overrides"},
		{ID: roleID("admin"), Name: string(types.RoleAdmin), Description: "Administrator — manages users, zones, and fee configs"},
		{ID: roleID("owner"), Name: string(types.RoleOwner), Description: "Business owner — full read access + holiday rate management"},
		{ID: roleID("engineer"), Name: string(types.RoleEngineer), Description: "Engineer — audit log access and system configuration"},
	}

	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&roles).Error
}

func seedPermissions(db *gorm.DB) error {
	perms := []authDomain.Permission{
		{ID: permID("gate.override"), Node: string(types.PermGateOverride), Description: "Manually open gate or override exit"},
		{ID: permID("fee.edit"), Node: string(types.PermFeeEdit), Description: "Create and modify fee configurations and tiers"},
		{ID: permID("report.view"), Node: string(types.PermReportView), Description: "View revenue reports and transaction summaries"},
		{ID: permID("user.manage"), Node: string(types.PermUserManage), Description: "Create, deactivate, and assign roles to users"},
		{ID: permID("zone.manage"), Node: string(types.PermZoneManage), Description: "Create and configure zones and gates"},
		{ID: permID("rfid.manage"), Node: string(types.PermRFIDManage), Description: "Deactivate RFID cards and manage card registry"},
		{ID: permID("audit.read"), Node: string(types.PermAuditRead), Description: "Read audit logs and export audit data"},
		{ID: permID("config.edit"), Node: string(types.PermConfigEdit), Description: "Edit system configs (OCR threshold, override limits)"},
	}

	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&perms).Error
}

func seedRolePermissions(db *gorm.DB) error {
	matrix := map[string][]string{
		"operator": {
			"gate.override",
		},
		"admin": {
			"gate.override",
			"fee.edit",
			"report.view",
			"user.manage",
			"zone.manage",
			"rfid.manage",
			"config.edit",
		},
		"owner": {
			"report.view",
			"fee.edit",
			"zone.manage",
			"config.edit",
		},
		"engineer": {
			"audit.read",
			"config.edit",
		},
	}

	var rps []authDomain.RolePermission
	now := time.Now()

	for roleName, permNodes := range matrix {
		for _, node := range permNodes {
			rID := roleID(roleName)
			pID := permID(node)
			rps = append(rps, authDomain.RolePermission{
				ID:           deterministicUUID(roleName + ":" + node),
				RoleID:       rID,
				PermissionID: pID,
				GrantedAt:    now,
			})
		}
	}

	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rps).Error
}

func seedAdminUser(db *gorm.DB) error {
	var count int64
	db.Model(&authDomain.User{}).Where("email = ?", "admin@parkieee.local").Count(&count)
	if count > 0 {
		return nil // already seeded
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("Admin@123!"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}

	admin := &authDomain.User{
		ID:           deterministicUUID("seed:admin"),
		Name:         "System Admin",
		Email:        "admin@parkieee.local",
		PasswordHash: string(hash),
		RoleID:       roleID("admin"),
		IsActive:     true,
	}

	return db.Create(admin).Error
}

var seedNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // uuid.NamespaceDNS

func deterministicUUID(name string) uuid.UUID {
	return uuid.NewSHA1(seedNamespace, []byte(name))
}

func roleID(name string) uuid.UUID {
	return deterministicUUID("role:" + name)
}

func permID(node string) uuid.UUID {
	return deterministicUUID("perm:" + node)
}
