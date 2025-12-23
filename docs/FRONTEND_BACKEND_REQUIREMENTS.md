# Frontend-Backend Requirements Analysis

## Summary des modifications Frontend

Toutes les modifications faites au frontend sont **purement UI/UX** et ne nécessitent **PAS** de modifications backend.

### Modifications réalisées:

#### 1. **Dashboard - Drag-and-Drop (Amélioration)**
- **Type**: UI improvement uniquement
- **Impact Backend**: ❌ AUCUN
- **Description**: Amélioration de la réactivité du drag-and-drop avec width responsif
- **Endpoints utilisés**: Aucun nouveau endpoint (utilise les endpoints existants via `DashboardGrid`)

#### 2. **Dashboard - Notification Bell (Intégration)**
- **Type**: UI component uniquement
- **Impact Backend**: ❌ AUCUN
- **Description**: Intégration du NotificationCenter (gestion en frontend avec Zustand store)
- **Endpoints utilisés**: Aucun (state management côté client uniquement)
- **Note**: Les notifications sont gérées localement - le backend peut envoyer les notifications via WebSocket ou polling dans le futur

#### 3. **Risks - View Toggle & Card View**
- **Type**: UI/UX improvement
- **Impact Backend**: ❌ AUCUN
- **Description**: Ajout du toggle Table/Card view et sauvegarde de la préférence en localStorage
- **Endpoints existants utilisés**:
  - `GET /risks` (already exists)
  - Aucun nouveau endpoint requis

#### 4. **Incidents - View Toggle & Card View**
- **Type**: UI/UX improvement
- **Impact Backend**: ❌ AUCUN
- **Endpoints existants utilisés**:
  - `GET /incidents` (already exists)
  - Aucun nouveau endpoint requis

#### 5. **Assets - View Toggle & Card View**
- **Type**: UI/UX improvement
- **Impact Backend**: ❌ AUCUN
- **Endpoints existants utilisés**:
  - `GET /assets` (already exists)
  - Aucun nouveau endpoint requis

#### 6. **Settings - Profile Personalization**
- **Type**: UI improvement + potential backend integration
- **Impact Backend**: ⚠️ OPTIONNEL (nécessite intégration backend si sauvegarde complète)
- **Description**: Ajout des champs: bio, phone, department, timezone
- **Endpoints existants utilisés**:
  - `PATCH /users/{userId}` (modification du profil) - **À IMPLÉMENTER si non existant**
- **Champs supplémentaires à supporter**:
  ```json
  {
    "bio": "string",
    "phone": "string",
    "department": "string",
    "timezone": "string"
  }
  ```

#### 7. **Settings - Team Management**
- **Type**: Feature UI + backend integration
- **Impact Backend**: ✅ NÉCESSITE DES ENDPOINTS
- **Description**: Création et gestion de teams
- **Endpoints à implémenter**:
  - `POST /teams` - Créer une team
  - `GET /teams` - Lister les teams
  - `DELETE /teams/{teamId}` - Supprimer une team
  - `PATCH /teams/{teamId}` - Modifier une team
  - `POST /teams/{teamId}/members` - Ajouter un membre
  - `DELETE /teams/{teamId}/members/{memberId}` - Retirer un membre

#### 8. **Settings - Integration Testing**
- **Type**: UI feature
- **Impact Backend**: ⚠️ OPTIONNEL (améliore UX si implémenté)
- **Description**: Ajout d'un bouton "Test" pour tester les connexions aux intégrations
- **Endpoints à implémenter (optionnel)**:
  - `POST /integrations/{integrationId}/test` - Tester une intégration

#### 9. **Users - Create User Modal**
- **Type**: Feature UI + backend integration
- **Impact Backend**: ✅ NÉCESSITE ENDPOINT
- **Description**: Création de nouveaux utilisateurs via modal admin
- **Endpoints utilisés**:
  - `POST /users` - **À IMPLÉMENTER si non existant**
  - Champs requis:
    ```json
    {
      "email": "string",
      "full_name": "string",
      "username": "string",
      "password": "string",
      "role": "viewer|analyst|admin",
      "group": "string (optionnel)"
    }
    ```

---

## Endpoints Backend Existants (Utilisés par les modifs)

Ces endpoints sont supposés exister car ils sont déjà utilisés dans le code:

### Authentication & Users
- ✅ `POST /auth/register`
- ✅ `POST /auth/login`
- ✅ `GET /users`
- ✅ `PATCH /users/{userId}/status`
- ✅ `PATCH /users/{userId}/role`
- ✅ `DELETE /users/{userId}`
- ❌ `POST /users` (Nouveau - à implémenter pour CreateUserModal)
- ❌ `PATCH /users/{userId}` (Nouveau - pour profile update)

### Risks
- ✅ `GET /risks`
- ✅ Autres endpoints risques (déjà existants)

### Incidents
- ✅ `GET /incidents`
- ✅ Autres endpoints incidents (déjà existants)

### Assets
- ✅ `GET /assets`
- ✅ Autres endpoints assets (déjà existants)

### Stats & Dashboard
- ✅ `GET /stats/risk-matrix`
- ✅ `GET /stats/trends`
- ✅ `GET /analytics/dashboard`

### Tokens
- ✅ `GET /tokens`
- ✅ `POST /tokens`
- ✅ `POST /tokens/{tokenId}/revoke`
- ✅ `DELETE /tokens/{tokenId}`
- ✅ `POST /tokens/{tokenId}/rotate`

---

## Endpoints À IMPLÉMENTER (Priorité)

### 🔴 Priorité HAUTE (Bloquent les features)

1. **POST /users** - Créer un nouvel utilisateur
   ```bash
   POST /api/v1/users
   Content-Type: application/json
   
   {
     "email": "user@example.com",
     "full_name": "John Doe",
     "username": "johndoe",
     "password": "securepassword",
     "role": "analyst",
     "group": "Security Team"
   }
   
   Response:
   {
     "id": "uuid",
     "email": "user@example.com",
     "username": "johndoe",
     "full_name": "John Doe",
     "role": "analyst",
     "is_active": true,
     "created_at": "2024-12-22T..."
   }
   ```

2. **PATCH /users/{userId}** - Mettre à jour le profil utilisateur
   ```bash
   PATCH /api/v1/users/{userId}
   Content-Type: application/json
   
   {
     "full_name": "John Doe",
     "bio": "Security expert...",
     "phone": "+1-555-0000",
     "department": "Security",
     "timezone": "America/New_York"
   }
   
   Response: { user_object }
   ```

### 🟡 Priorité MOYENNE (Améliore UX)

3. **POST /teams** - Créer une team
   ```bash
   POST /api/v1/teams
   
   {
     "name": "Security Team",
     "description": "Main security operations team"
   }
   
   Response:
   {
     "id": "uuid",
     "name": "Security Team",
     "description": "Main security operations team",
     "members": 0,
     "created_at": "2024-12-22T..."
   }
   ```

4. **GET /teams** - Lister les teams
   ```bash
   GET /api/v1/teams
   
   Response:
   {
     "data": [
       { "id": "...", "name": "...", "members": 5, ... }
     ]
   }
   ```

5. **DELETE /teams/{teamId}** - Supprimer une team
   ```bash
   DELETE /api/v1/teams/{teamId}
   ```

6. **POST /integrations/{integrationId}/test** - Tester une intégration
   ```bash
   POST /api/v1/integrations/{integrationId}/test
   
   {
     "api_url": "https://...",
     "api_key": "..."
   }
   
   Response:
   {
     "success": true,
     "message": "Connection successful"
   }
   ```

---

## Recommandations

### ✅ À faire maintenant:
1. Implémenter `POST /users` pour le CreateUserModal
2. Implémenter `PATCH /users/{userId}` pour le profile update
3. Ces deux endpoints sont critiques pour la fonctionnalité

### ⏳ À faire ensuite:
1. Implémenter les endpoints Teams (`POST`, `GET`, `DELETE`)
2. Implémenter le test des intégrations (optionnel mais améliore UX)

### 💡 Notes importantes:
- **Aucune modification** de la base de données n'est strictement requise pour la majorité des features
- Les champs `bio`, `phone`, `department`, `timezone` doivent être ajoutés au modèle `User` si pas déjà présents
- Les tables `teams` et `team_members` doivent être créées pour la gestion des teams
- Toutes les modifications frontend sont **non-breaking** et compatibles avec le backend existant

---

## Vérification rapide du backend

Pour vérifier quels endpoints existent déjà, vous pouvez:

```bash
# Lister tous les routes Go
grep -r "router\." backend/cmd/server/ backend/internal/handlers/

# Ou vérifier directement les handler definitions
ls backend/internal/handlers/
```

---

**Générée le**: 22 Décembre 2025
**Status**: Analyse complète des dépendances Frontend ✅
