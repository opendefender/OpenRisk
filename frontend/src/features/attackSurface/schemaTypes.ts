// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Contract-first: aliases onto the types generated from docs/openapi.yaml.
import type { components } from '../../types/openapi.generated';

export type AssetTypeSchema = components['schemas']['AssetTypeSchema'];
export type AttributeDef = components['schemas']['AttributeDef'];
export type AttributeType = NonNullable<AttributeDef['type']>;
export type AssetCategory = NonNullable<AssetTypeSchema['category']>;
export type UpdateAssetTypeSchemaInput = components['schemas']['UpdateAssetTypeSchemaInput'];

/** One typed attribute value, as it travels to and from the API. */
export type AttributeValue = string | number | boolean | string[];
export type AttributeBag = Record<string, AttributeValue>;

/** Canonical order — the same one the server returns. */
export const ASSET_CATEGORIES: AssetCategory[] = [
  'server',
  'workstation',
  'application',
  'database',
  'network',
  'cloud',
  'vendor',
  'data_processing',
];

/** Every attribute type the schema editor can offer. */
export const ATTRIBUTE_TYPES: AttributeType[] = [
  'string',
  'text',
  'number',
  'integer',
  'boolean',
  'enum',
  'multi_enum',
  'date',
  'ip',
  'ip_list',
  'hostname',
  'cidr',
  'url',
  'email',
  'string_list',
];

/** Types whose value is a list. Mirrors AttributeType.IsList() on the server. */
export function isListType(t: AttributeType | undefined): boolean {
  return t === 'multi_enum' || t === 'ip_list' || t === 'string_list';
}

export const CATEGORY_LABELS: Record<AssetCategory, string> = {
  server: 'Serveur',
  workstation: 'Poste de travail',
  application: 'Application',
  database: 'Base de données',
  network: 'Équipement réseau',
  cloud: 'Ressource cloud',
  vendor: 'Fournisseur / tiers',
  data_processing: 'Données / traitement',
};

export const ATTRIBUTE_TYPE_LABELS: Record<AttributeType, string> = {
  string: 'Texte court',
  text: 'Texte long',
  number: 'Nombre',
  integer: 'Nombre entier',
  boolean: 'Oui / non',
  enum: 'Liste de choix',
  multi_enum: 'Choix multiples',
  date: 'Date',
  ip: 'Adresse IP',
  ip_list: 'Liste d’adresses IP',
  hostname: 'Nom d’hôte',
  cidr: 'Plage CIDR',
  url: 'URL',
  email: 'Adresse e-mail',
  string_list: 'Liste de valeurs',
};
