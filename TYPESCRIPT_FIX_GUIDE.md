# 🔧 Guide Rapide - Fix TypeScript Errors (5 min)

## Problème
Le fichier `frontend/src/pages/Reports.tsx` a 5 erreurs TypeScript qui bloquent la compilation.

## Localisation des Erreurs

```
Lignes 200, 207, 214, 223: Property 'size' does not exist on Button
Ligne 239: variant="outline" is invalid (expected: "primary" | "secondary" | "ghost")
```

## Solution Rapide (Option 1: Retirer les props invalides)

### Étape 1: Ouvrir le fichier
```bash
cd frontend
code src/pages/Reports.tsx
```

### Étape 2: Fixer chaque erreur

**Erreur 1-4** (Lignes 200, 207, 214, 223):
```typescript
// AVANT ❌
<Button variant="ghost" size="sm" title="..." />

// APRÈS ✅
<Button variant="ghost" title="..." />
```

**Erreur 5** (Ligne 239):
```typescript
// AVANT ❌
<Button variant="outline">Add Scheduled Report</Button>

// APRÈS ✅
<Button variant="secondary">Add Scheduled Report</Button>
```

### Étape 3: Valider la fix
```bash
npm run build
# ✅ Build should succeed
```

## Vérifier le Button Component

Si vous voulez vérifier les props valides du Button:

```bash
# Trouver le Button component
find src -name "*Button*" -type f
# Probablement: src/components/Button.tsx ou src/components/ui/Button.tsx
```

Puis vérifier les props exportées:
```typescript
interface ButtonProps {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  // Note: NO 'size' prop defined
}
```

## Après la Fix

```bash
# 1. Tester la compilation
npm run build

# 2. Tester localement
npm run dev

# 3. Commit
git add frontend/src/pages/Reports.tsx
git commit -m "fix: Correct TypeScript errors in Reports.tsx - remove invalid props"
git push origin stag
```

## Alternative (Si vous voulez garder size="sm")

Si le design requiert vraiment une taille "sm", vous pouvez:

1. **Ajouter la prop au Button component**:
```typescript
// src/components/Button.tsx
interface ButtonProps {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg";  // ← Ajouter cette ligne
}
```

2. **Ajouter le CSS correspondant**:
```typescript
const sizeClasses = {
  sm: "px-2 py-1 text-sm",
  md: "px-3 py-2 text-base",
  lg: "px-4 py-3 text-lg",
};
```

Mais pour un déploiement rapide, **l'option 1 (retirer les props) est plus simple**.

---

## Estimated Time: ⏱️ 5 minutes maximum
