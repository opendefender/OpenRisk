  IMMEDIATE ACTION: Fix Frontend Build Errors (- Hours)

Status:  TypeScript errors blocking deployment  
Priority: CRITICAL (Must fix before Phase  work)  
Effort: - hours ( developer)  
Outcome: Clean build, ready for Storybook setup  

---

  Error Breakdown

 Category : Readonly Array Type Mismatches ( errors)
Files: roleTemplateUtils.ts (), rbacTestUtils.ts (), others ()  
Issue: Trying to assign readonly string[] to string[]  
Fix: Cast or convert to mutable array

 Category : Button Variant Type ( errors)
Files: ThreatMap.tsx (), RoleManagement.tsx (), others ()  
Issue: Using variant="outline" but type only allows "primary|secondary|ghost|danger"  
Fix: Change "outline" to "ghost"

 Category : Missing Types & Unused Imports ( errors)
Files: permissionAuditLog.ts (), permissionCache.ts (), others ()  
Issue: Missing @types/node or unused variable declarations  
Fix: Add type definitions or remove unused imports

---

  Quick Fix Steps

 Step : Add Missing Type Definitions
bash
cd frontend
npm install --save-dev @types/node


Modified: package.json  
Result: Fixes  errors in permissionAuditLog.ts and permissionCache.ts

---

 Step : Fix Readonly Array Issues

File: [frontend/src/utils/roleTemplateUtils.ts](src/utils/roleTemplateUtils.ts)

Current issue: Trying to cast readonly arrays to mutable

Solution: 
typescript
// Change this:
const templates: RoleTemplate[] = ROLE_TEMPLATES;

// To this:
const templates: RoleTemplate[] = ROLE_TEMPLATES.map(t => ({...t}));
// OR
const templates = [...ROLE_TEMPLATES] as RoleTemplate[];


Affected Lines: , , , , and  more in this file

---

 Step : Fix Button Variant Issues ( errors)

Search: variant="outline"  
Replace: variant="ghost"

Files to Update:
- [src/pages/ThreatMap.tsx](src/pages/ThreatMap.tsx) - Line 
- [src/pages/RoleManagement.tsx](src/pages/RoleManagement.tsx) - Line 
- [src/pages/TenantManagement.tsx](src/pages/TenantManagement.tsx) - Line 
- [src/features/settings/GeneralTab.tsx](src/features/settings/GeneralTab.tsx) - Check for outline
- [src/features/settings/TeamTab.tsx](src/features/settings/TeamTab.tsx) - Check for outline

---

 Step : Remove Unused Imports & Variables

Commands:
bash
 Find unused imports
grep -r "import.never read" src/

 Fix specific files:
 . src/components/rbac/RoleTemplateBuilder.tsx (line : 'getRecommendedTemplate')
 . src/components/dashboard/RBACDashboardWidget.tsx (line : 'Lock')
 . src/pages/RoleManagement.tsx (line : 'selectedPermissions')
 . src/pages/ThreatMap.tsx (line , : 'isLoading', 'error', 'totalThreats')


---

 Step : Fix Missing Module Imports

File: [src/components/rbac/PermissionRoutes.tsx](src/components/rbac/PermissionRoutes.tsx)

Error: Cannot find module '../store/authStore'

Check: 
- Does authStore exist at src/store/authStore.ts?
- Check import path correctness
- Verify file exists or create it

Similar issues:
- Line : '../hooks/usePermissions' - verify path
- Line : '../utils/rbacHelpers' - verify path

---

  Complete Fix Checklist

 Quick Wins ( minutes)
- [ ] Add @types/node to package.json
- [ ] Replace all variant="outline" with variant="ghost" ( locations)
- [ ] Build & verify reduction in error count

 Medium Effort ( hour)
- [ ] Fix readonly array casts in roleTemplateUtils.ts ( errors)
- [ ] Fix readonly array casts in rbacTestUtils.ts ( errors)
- [ ] Build & verify

 Remaining Cleanup ( hour)
- [ ] Remove unused imports (grep for unused)
- [ ] Fix module path issues (verify imports exist)
- [ ] Remove unused variables from functions
- [ ] Final build test

---

  Execution Commands

 Phase : Add Missing Dependencies
bash
cd frontend
npm install --save-dev @types/node


 Phase : Bulk Replace Button Variant
bash
 Find all outline variants
grep -rn 'variant="outline"' src/

 Manual replace in each file (or use sed if comfortable)
 sed -i 's/variant="outline"/variant="ghost"/g' src//.tsx


 Phase : Build Check
bash
npm run build >& | grep "error TS" | wc -l
 Should reduce from  to ~-


 Phase : Fix Array Casts
- Edit src/utils/roleTemplateUtils.ts 
- Edit src/utils/rbacTestUtils.ts
- Replace readonly array assignments

 Phase : Final Cleanup
bash
npm run build >& | grep "error TS"
 Resolve remaining errors one by one


 Phase : Success!
bash
npm run build
 Should see:   modules built successfully
npm run preview   Test production build


---

  Expected Timeline

| Phase | Task | Time | Result |
|-------|------|------|--------|
|  | Add @types/node |  min | - errors |
|  | Fix variant="outline" |  min | - errors |
|  | Fix readonly arrays |  min | - errors |
|  | Remove unused imports |  min | - errors |
|  | Fix module paths |  min | - errors |
| Total | Build Fixes |  min |   errors |

---

  Notes

- After fixes, next step is Storybook setup (Design System)
- These fixes enable clean builds for both dev and production
- Testing should pass after these fixes (existing test files already excluded)
- Frontend dev server (npm run dev) should work immediately after @types/node

---

  Sign-Off Criteria

Build is "fixed" when:
bash
npm run build
 Output:   modules built successfully
 Exit code: 
 No error messages


bash
npm run preview  
 Opens http://localhost:
 App loads without console errors


---

Ready to fix the build? Start with Step ! 
