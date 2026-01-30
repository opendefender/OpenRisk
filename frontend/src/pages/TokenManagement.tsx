import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Key, Trash, Lock, Copy, Plus, Search, Calendar, ChevronDown } from 'lucide-react';
import { api } from '../lib/api';
import { toast } from 'sonner';
import { Button } from '../components/ui/Button';

interface APIToken {
  id: string;
  name: string;
  description?: string;
  token_prefix?: string;
  status: string;
  permissions?: string[];
  scopes?: string[];
  expires_at?: string;
  created_at: string;
  last_used_at?: string;
}

interface CreateTokenRequest {
  name: string;
  description: string;
  permissions?: string[];
  scopes?: string[];
  expires_at?: string;
}

export const TokenManagement = () => {
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isCreating, setIsCreating] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newTokenValue, setNewTokenValue] = useState<string>('');
  const [formData, setFormData] = useState<CreateTokenRequest>({
    name: '',
    description: '',
  });

  useEffect(() => {
    fetchTokens();
  }, []);

  const fetchTokens = async () => {
    setIsLoading(true);
    try {
      const response = await api.get('/tokens');
      setTokens(response.data || []);
    } catch (err: any) {
      toast.error('Unable to load your API tokens. Please refresh the page.');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleCreateToken = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim()) {
      toast.error('Token name is required');
      return;
    }

    setIsCreating(true);
    try {
      const response = await api.post('/tokens', {
        name: formData.name,
        description: formData.description,
        permissions: formData.permissions || [],
        scopes: formData.scopes || [],
        expires_at: formData.expires_at,
      });

      setNewTokenValue(response.data.token_value);
      setFormData({ name: '', description: '' });
      await fetchTokens();
      toast.success('Token created successfully. Copy the token value now - you won\'t see it again!');
    } catch (err: any) {
      toast.error(err.response?.data?.message || "Couldn't create the API token. Please check your input and try again.");
    } finally {
      setIsCreating(false);
    }
  };

  const handleRevokeToken = async (tokenId: string) => {
    if (!confirm('Are you sure you want to revoke this token? It will no longer be usable.')) {
      return;
    }

    try {
      await api.post(/tokens/${tokenId}/revoke);
      setTokens(tokens.map(t => t.id === tokenId ? { ...t, status: 'revoked' } : t));
      toast.success('Token revoked');
    } catch (err) {
      toast.error("We couldn't disable this token. Please try again.");
    }
  };

  const handleDeleteToken = async (tokenId: string) => {
    if (!confirm('Are you sure you want to permanently delete this token?')) {
      return;
    }

    try {
      await api.delete(/tokens/${tokenId});
      setTokens(tokens.filter(t => t.id !== tokenId));
      toast.success('Token deleted');
    } catch (err) {
      toast.error("We couldn't remove this token. Please try again.");
    }
  };

  const handleRotateToken = async (tokenId: string) => {
    if (!confirm('This will create a new token and keep the old one. Continue?')) {
      return;
    }

    try {
      const response = await api.post(/tokens/${tokenId}/rotate);
      setNewTokenValue(response.data.new_token_value);
      await fetchTokens();
      toast.success('Token rotated successfully. Copy the new token value now!');
    } catch (err) {
      toast.error("We couldn't refresh the token. Please try again.");
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  const filteredTokens = tokens.filter(token => 
    token.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const activeTokens = filteredTokens.filter(t => t.status === 'active').length;

  const formatDate = (dateString?: string) => {
    if (!dateString) return 'Never';
    return new Date(dateString).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };

  const isExpiringSoon = (expiresAt?: string) => {
    if (!expiresAt) return false;
    const expiryDate = new Date(expiresAt);
    const daysUntilExpiry = Math.floor((expiryDate.getTime() - Date.now()) / (      ));
    return daysUntilExpiry <=  && daysUntilExpiry > ;
  };

  const isExpired = (expiresAt?: string) => {
    if (!expiresAt) return false;
    return new Date(expiresAt) < new Date();
  };

  return (
    <div className="min-h-screen bg-zinc-">
      {/ Header /}
      <div className="border-b border-zinc- bg-gradient-to-b from-zinc- to-zinc- p-">
        <div className="max-w-xl mx-auto">
          <div className="flex items-center gap- mb-">
            <Key className="w- h- text-amber-" />
            <h className="text-xl font-bold text-white">API Tokens</h>
          </div>
          <p className="text-zinc-">Manage your API tokens for programmatic access</p>
        </div>
      </div>

      {/ Main Content /}
      <div className="max-w-xl mx-auto p-">
        {/ Stats Bar /}
        <div className="grid grid-cols- gap- mb-">
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            className="bg-zinc- border border-zinc- rounded-lg p-"
          >
            <div className="text-zinc- text-sm font-medium">Total Tokens</div>
            <div className="text-xl font-bold text-white mt-">{tokens.length}</div>
          </motion.div>
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ delay: . }}
            className="bg-zinc- border border-zinc- rounded-lg p-"
          >
            <div className="text-zinc- text-sm font-medium">Active</div>
            <div className="text-xl font-bold text-green- mt-">{activeTokens}</div>
          </motion.div>
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ delay: . }}
            className="bg-zinc- border border-zinc- rounded-lg p-"
          >
            <div className="text-zinc- text-sm font-medium">Revoked</div>
            <div className="text-xl font-bold text-red- mt-">
              {tokens.filter(t => t.status === 'revoked').length}
            </div>
          </motion.div>
        </div>

        {/ New Token Display /}
        {newTokenValue && (
          <motion.div
            initial={{ opacity: , scale: . }}
            animate={{ opacity: , scale:  }}
            className="mb- bg-green-/ border border-green-/ rounded-lg p-"
          >
            <div className="flex items-start justify-between">
              <div className="flex-">
                <h className="font-semibold text-green- mb-">New Token Created</h>
                <p className="text-sm text-zinc- mb-">Save this token now. You won't be able to see it again!</p>
                <div className="bg-zinc- rounded px- py- flex items-center justify-between">
                  <code className="text-sm text-white break-all">{newTokenValue}</code>
                  <button
                    onClick={() => copyToClipboard(newTokenValue)}
                    className="ml- p- hover:bg-zinc- rounded"
                  >
                    <Copy className="w- h- text-green-" />
                  </button>
                </div>
              </div>
              <button
                onClick={() => setNewTokenValue('')}
                className="text-zinc- hover:text-white ml-"
              >
                
              </button>
            </div>
          </motion.div>
        )}

        {/ Controls /}
        <div className="flex gap- mb-">
          <Button
            onClick={() => setShowCreateForm(!showCreateForm)}
            className="gap-"
          >
            <Plus className="w- h-" />
            New Token
          </Button>
        </div>

        {/ Create Form /}
        {showCreateForm && (
          <motion.div
            initial={{ opacity: , y: - }}
            animate={{ opacity: , y:  }}
            className="mb- bg-zinc- border border-zinc- rounded-lg p-"
          >
            <form onSubmit={handleCreateToken} className="space-y-">
              <div>
                <label className="block text-sm font-medium text-zinc- mb-">
                  Token Name 
                </label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder="e.g., CI/CD Pipeline, DataSync"
                  className="w-full bg-zinc- border border-zinc- rounded px- py- text-white text-sm focus:outline-none focus:border-blue-"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc- mb-">
                  Description
                </label>
                <textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  placeholder="What is this token for?"
                  className="w-full bg-zinc- border border-zinc- rounded px- py- text-white text-sm focus:outline-none focus:border-blue- resize-none"
                  rows={}
                />
              </div>
              <div className="flex gap-">
                <Button type="submit" disabled={isCreating} className="gap-">
                  {isCreating ? 'Creating...' : 'Create Token'}
                </Button>
                <Button
                  type="button"
                  onClick={() => setShowCreateForm(false)}
                  variant="secondary"
                >
                  Cancel
                </Button>
              </div>
            </form>
          </motion.div>
        )}

        {/ Search /}
        <div className="mb- flex items-center gap-">
          <Search className="w- h- text-zinc-" />
          <input
            type="text"
            placeholder="Search tokens..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="flex- bg-zinc- border border-zinc- rounded px- py- text-white text-sm focus:outline-none focus:border-blue-"
          />
        </div>

        {/ Tokens List /}
        {isLoading ? (
          <div className="text-center py-">
            <p className="text-zinc-">Loading tokens...</p>
          </div>
        ) : filteredTokens.length ===  ? (
          <div className="text-center py-">
            <Key className="w- h- text-zinc- mx-auto mb-" />
            <p className="text-zinc-">No tokens found</p>
          </div>
        ) : (
          <div className="space-y-">
            {filteredTokens.map((token, index) => (
              <motion.div
                key={token.id}
                initial={{ opacity: , y:  }}
                animate={{ opacity: , y:  }}
                transition={{ delay: index  . }}
                className="bg-zinc- border border-zinc- rounded-lg p- hover:border-zinc- transition"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-">
                    <div className="flex items-center gap- mb-">
                      <h className="font-semibold text-white">{token.name}</h>
                      <span className={text-xs px- py- rounded font-medium ${
                        token.status === 'active'
                          ? 'bg-green-/ text-green- border border-green-/'
                          : 'bg-red-/ text-red- border border-red-/'
                      }}>
                        {token.status.charAt().toUpperCase() + token.status.slice()}
                      </span>
                      {token.token_prefix && (
                        <span className="text-xs text-zinc- font-mono">{token.token_prefix}...</span>
                      )}
                    </div>
                    {token.description && (
                      <p className="text-sm text-zinc- mb-">{token.description}</p>
                    )}
                    <div className="flex gap- text-xs text-zinc-">
                      <div className="flex items-center gap-">
                        <Calendar className="w- h-" />
                        Created: {formatDate(token.created_at)}
                      </div>
                      {token.last_used_at && (
                        <div className="flex items-center gap-">
                          <Calendar className="w- h-" />
                          Last used: {formatDate(token.last_used_at)}
                        </div>
                      )}
                      {token.expires_at && (
                        <div className={flex items-center gap- ${
                          isExpired(token.expires_at) ? 'text-red-' :
                          isExpiringSoon(token.expires_at) ? 'text-amber-' : ''
                        }}>
                          <Lock className="w- h-" />
                          Expires: {formatDate(token.expires_at)}
                          {isExpiringSoon(token.expires_at) && ' (Soon)'}
                          {isExpired(token.expires_at) && ' (Expired)'}
                        </div>
                      )}
                    </div>
                    {(token.permissions && token.permissions.length > ) && (
                      <div className="mt-">
                        <p className="text-xs text-zinc- mb-">Permissions:</p>
                        <div className="flex flex-wrap gap-">
                          {token.permissions.map((perm) => (
                            <span
                              key={perm}
                              className="text-xs bg-blue-/ text-blue- border border-blue-/ rounded px- py-"
                            >
                              {perm}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap- ml-">
                    {token.status === 'active' && (
                      <>
                        <button
                          onClick={() => handleRotateToken(token.id)}
                          className="p- text-blue- hover:bg-blue-/ rounded transition"
                          title="Rotate token"
                        >
                          <ChevronDown className="w- h-" />
                        </button>
                        <button
                          onClick={() => handleRevokeToken(token.id)}
                          className="p- text-amber- hover:bg-amber-/ rounded transition"
                          title="Revoke token"
                        >
                          <Lock className="w- h-" />
                        </button>
                      </>
                    )}
                    <button
                      onClick={() => handleDeleteToken(token.id)}
                      className="p- text-red- hover:bg-red-/ rounded transition"
                      title="Delete token"
                    >
                      <Trash className="w- h-" />
                    </button>
                  </div>
                </div>
              </motion.div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};
