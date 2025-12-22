import { useState } from 'react';
import { Shield, Plus, Trash2, Users, Edit2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { toast } from 'sonner';
import { motion } from 'framer-motion';

interface Team {
  id: string;
  name: string;
  members: number;
  description: string;
  createdAt: string;
}

interface TeamMember {
  id: string;
  name: string;
  email: string;
  role: string;
  status: 'active' | 'inactive';
  avatar: string;
}

export const TeamTab = () => {
    const [isAdmin, setIsAdmin] = useState(true); // Should come from auth store
    const [teams, setTeams] = useState<Team[]>([
        {
            id: '1',
            name: 'Security Team',
            members: 5,
            description: 'Main security operations team',
            createdAt: '2024-01-15'
        },
        {
            id: '2',
            name: 'Compliance Team',
            members: 3,
            description: 'Compliance and audit team',
            createdAt: '2024-02-20'
        }
    ]);
    
    const [members, setMembers] = useState<TeamMember[]>([
        {
            id: '1',
            name: 'System Admin',
            email: 'admin@opendefender.io',
            role: 'ADMIN',
            status: 'active',
            avatar: 'A'
        },
        {
            id: '2',
            name: 'Security Lead',
            email: 'security@opendefender.io',
            role: 'ANALYST',
            status: 'active',
            avatar: 'S'
        }
    ]);

    const [showCreateTeam, setShowCreateTeam] = useState(false);
    const [newTeamName, setNewTeamName] = useState('');
    const [newTeamDesc, setNewTeamDesc] = useState('');

    const handleCreateTeam = async () => {
        if (!newTeamName.trim()) {
            toast.error('Team name is required');
            return;
        }
        
        try {
            const newTeam: Team = {
                id: Date.now().toString(),
                name: newTeamName,
                members: 0,
                description: newTeamDesc,
                createdAt: new Date().toISOString().split('T')[0]
            };
            setTeams([...teams, newTeam]);
            setNewTeamName('');
            setNewTeamDesc('');
            setShowCreateTeam(false);
            toast.success('Team created successfully');
        } catch (error) {
            toast.error('Failed to create team');
        }
    };

    const handleDeleteTeam = async (teamId: string) => {
        try {
            setTeams(teams.filter(t => t.id !== teamId));
            toast.success('Team deleted successfully');
        } catch (error) {
            toast.error('Failed to delete team');
        }
    };

    const handleRemoveMember = async (memberId: string) => {
        try {
            setMembers(members.filter(m => m.id !== memberId));
            toast.success('Member removed from team');
        } catch (error) {
            toast.error('Failed to remove member');
        }
    };

    return (
        <div className="space-y-8">
            {/* Teams Section */}
            {isAdmin && (
                <div className="space-y-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <h3 className="text-2xl font-bold text-white mb-1">Teams</h3>
                            <p className="text-zinc-400 text-sm">Create and manage teams for better organization.</p>
                        </div>
                        <Button onClick={() => setShowCreateTeam(!showCreateTeam)}>
                            <Plus size={16} className="mr-2" /> Create Team
                        </Button>
                    </div>

                    {showCreateTeam && (
                        <motion.div
                            initial={{ opacity: 0, height: 0 }}
                            animate={{ opacity: 1, height: 'auto' }}
                            className="bg-surface border border-border rounded-lg p-6 space-y-4"
                        >
                            <input
                                type="text"
                                placeholder="Team name (e.g., Security Team)"
                                className="w-full bg-zinc-900 border border-border rounded-lg px-4 py-2 text-sm text-white placeholder:text-zinc-600 focus:ring-2 focus:ring-primary/50 outline-none"
                                value={newTeamName}
                                onChange={(e) => setNewTeamName(e.target.value)}
                            />
                            <textarea
                                placeholder="Team description..."
                                className="w-full bg-zinc-900 border border-border rounded-lg px-4 py-2 text-sm text-white placeholder:text-zinc-600 focus:ring-2 focus:ring-primary/50 outline-none resize-none"
                                rows={3}
                                value={newTeamDesc}
                                onChange={(e) => setNewTeamDesc(e.target.value)}
                            />
                            <div className="flex gap-2">
                                <Button onClick={handleCreateTeam}>Create</Button>
                                <Button variant="ghost" onClick={() => setShowCreateTeam(false)}>Cancel</Button>
                            </div>
                        </motion.div>
                    )}

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        {teams.map((team) => (
                            <motion.div
                                key={team.id}
                                initial={{ opacity: 0, y: 20 }}
                                animate={{ opacity: 1, y: 0 }}
                                className="bg-surface border border-border rounded-lg p-6 hover:border-primary/50 transition-all group"
                            >
                                <div className="flex items-start justify-between mb-3">
                                    <div className="flex items-center gap-3">
                                        <div className="p-2 rounded-lg bg-primary/10">
                                            <Users size={20} className="text-primary" />
                                        </div>
                                        <div>
                                            <h4 className="font-semibold text-white group-hover:text-primary transition-colors">
                                                {team.name}
                                            </h4>
                                            <p className="text-xs text-zinc-500">Created {team.createdAt}</p>
                                        </div>
                                    </div>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => handleDeleteTeam(team.id)}
                                    >
                                        <Trash2 size={16} className="text-red-400" />
                                    </Button>
                                </div>
                                <p className="text-sm text-zinc-400 mb-4">{team.description}</p>
                                <div className="flex items-center justify-between pt-4 border-t border-border">
                                    <span className="text-xs text-zinc-500">{team.members} members</span>
                                    <Button variant="ghost" className="text-xs">Manage</Button>
                                </div>
                            </motion.div>
                        ))}
                    </div>
                </div>
            )}

            {/* Team Members Section */}
            <div className="space-y-4">
                <div>
                    <h3 className="text-2xl font-bold text-white mb-1">Team Members</h3>
                    <p className="text-zinc-400 text-sm">Manage access and roles for your team.</p>
                </div>
                
                <div className="bg-surface border border-border rounded-xl overflow-hidden">
                    <table className="w-full text-left text-sm">
                        <thead className="bg-white/5 text-zinc-400 font-medium uppercase text-xs">
                            <tr>
                                <th className="px-6 py-4">User</th>
                                <th className="px-6 py-4">Role</th>
                                <th className="px-6 py-4">Status</th>
                                {isAdmin && <th className="px-6 py-4 text-right">Actions</th>}
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/5">
                            {members.map((member) => (
                                <tr key={member.id} className="hover:bg-white/5 transition-colors">
                                    <td className="px-6 py-4">
                                        <div className="flex items-center gap-3">
                                            <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center font-bold text-white text-sm">
                                                {member.avatar}
                                            </div>
                                            <div>
                                                <div className="font-medium text-white">{member.name}</div>
                                                <div className="text-zinc-500 text-xs">{member.email}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-4">
                                        {isAdmin ? (
                                            <select className="bg-zinc-900 border border-border rounded px-2 py-1 text-xs text-white focus:ring-2 focus:ring-primary/50 outline-none">
                                                <option>ADMIN</option>
                                                <option>ANALYST</option>
                                                <option>VIEWER</option>
                                            </select>
                                        ) : (
                                            <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded bg-purple-500/10 text-purple-400 border border-purple-500/20 text-xs font-medium">
                                                <Shield size={10} /> {member.role}
                                            </span>
                                        )}
                                    </td>
                                    <td className="px-6 py-4">
                                        <span className={member.status === 'active' ? 'text-emerald-500 font-medium' : 'text-zinc-500'}>
                                            {member.status === 'active' ? 'Active' : 'Inactive'}
                                        </span>
                                    </td>
                                    {isAdmin && (
                                        <td className="px-6 py-4 text-right">
                                            <div className="flex gap-2 justify-end">
                                                <Button variant="ghost" size="sm" className="text-xs">
                                                    <Edit2 size={14} />
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="text-xs"
                                                    onClick={() => handleRemoveMember(member.id)}
                                                >
                                                    <Trash2 size={14} className="text-red-400" />
                                                </Button>
                                            </div>
                                        </td>
                                    )}
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
};