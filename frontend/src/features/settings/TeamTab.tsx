import { useState } from 'react';
import { Shield, Plus, Trash, Users, Edit } from 'lucide-react';
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
            id: '',
            name: 'Security Team',
            members: ,
            description: 'Main security operations team',
            createdAt: '--'
        },
        {
            id: '',
            name: 'Compliance Team',
            members: ,
            description: 'Compliance and audit team',
            createdAt: '--'
        }
    ]);
    
    const [members, setMembers] = useState<TeamMember[]>([
        {
            id: '',
            name: 'System Admin',
            email: 'admin@opendefender.io',
            role: 'ADMIN',
            status: 'active',
            avatar: 'A'
        },
        {
            id: '',
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
                members: ,
                description: newTeamDesc,
                createdAt: new Date().toISOString().split('T')[]
            };
            setTeams([...teams, newTeam]);
            setNewTeamName('');
            setNewTeamDesc('');
            setShowCreateTeam(false);
            toast.success('Team created successfully');
        } catch (error) {
            toast.error("We couldn't create the team. Please enter a valid team name and try again.");
        }
    };

    const handleDeleteTeam = async (teamId: string) => {
        try {
            setTeams(teams.filter(t => t.id !== teamId));
            toast.success('Team deleted successfully');
        } catch (error) {
            toast.error("We couldn't delete the team. Please try again or contact support.");
        }
    };

    const handleRemoveMember = async (memberId: string) => {
        try {
            setMembers(members.filter(m => m.id !== memberId));
            toast.success('Member removed from team');
        } catch (error) {
            toast.error("We couldn't remove the team member. Please try again.");
        }
    };

    return (
        <div className="space-y-">
            {/ Teams Section /}
            {isAdmin && (
                <div className="space-y-">
                    <div className="flex items-center justify-between">
                        <div>
                            <h className="text-xl font-bold text-white mb-">Teams</h>
                            <p className="text-zinc- text-sm">Create and manage teams for better organization.</p>
                        </div>
                        <Button onClick={() => setShowCreateTeam(!showCreateTeam)}>
                            <Plus size={} className="mr-" /> Create Team
                        </Button>
                    </div>

                    {showCreateTeam && (
                        <motion.div
                            initial={{ opacity: , height:  }}
                            animate={{ opacity: , height: 'auto' }}
                            className="bg-surface border border-border rounded-lg p- space-y-"
                        >
                            <input
                                type="text"
                                placeholder="Team name (e.g., Security Team)"
                                className="w-full bg-zinc- border border-border rounded-lg px- py- text-sm text-white placeholder:text-zinc- focus:ring- focus:ring-primary/ outline-none"
                                value={newTeamName}
                                onChange={(e) => setNewTeamName(e.target.value)}
                            />
                            <textarea
                                placeholder="Team description..."
                                className="w-full bg-zinc- border border-border rounded-lg px- py- text-sm text-white placeholder:text-zinc- focus:ring- focus:ring-primary/ outline-none resize-none"
                                rows={}
                                value={newTeamDesc}
                                onChange={(e) => setNewTeamDesc(e.target.value)}
                            />
                            <div className="flex gap-">
                                <Button onClick={handleCreateTeam}>Create</Button>
                                <Button variant="ghost" onClick={() => setShowCreateTeam(false)}>Cancel</Button>
                            </div>
                        </motion.div>
                    )}

                    <div className="grid grid-cols- md:grid-cols- gap-">
                        {teams.map((team) => (
                            <motion.div
                                key={team.id}
                                initial={{ opacity: , y:  }}
                                animate={{ opacity: , y:  }}
                                className="bg-surface border border-border rounded-lg p- hover:border-primary/ transition-all group"
                            >
                                <div className="flex items-start justify-between mb-">
                                    <div className="flex items-center gap-">
                                        <div className="p- rounded-lg bg-primary/">
                                            <Users size={} className="text-primary" />
                                        </div>
                                        <div>
                                            <h className="font-semibold text-white group-hover:text-primary transition-colors">
                                                {team.name}
                                            </h>
                                            <p className="text-xs text-zinc-">Created {team.createdAt}</p>
                                        </div>
                                    </div>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => handleDeleteTeam(team.id)}
                                    >
                                        <Trash size={} className="text-red-" />
                                    </Button>
                                </div>
                                <p className="text-sm text-zinc- mb-">{team.description}</p>
                                <div className="flex items-center justify-between pt- border-t border-border">
                                    <span className="text-xs text-zinc-">{team.members} members</span>
                                    <Button variant="ghost" className="text-xs">Manage</Button>
                                </div>
                            </motion.div>
                        ))}
                    </div>
                </div>
            )}

            {/ Team Members Section /}
            <div className="space-y-">
                <div>
                    <h className="text-xl font-bold text-white mb-">Team Members</h>
                    <p className="text-zinc- text-sm">Manage access and roles for your team.</p>
                </div>
                
                <div className="bg-surface border border-border rounded-xl overflow-hidden">
                    <table className="w-full text-left text-sm">
                        <thead className="bg-white/ text-zinc- font-medium uppercase text-xs">
                            <tr>
                                <th className="px- py-">User</th>
                                <th className="px- py-">Role</th>
                                <th className="px- py-">Status</th>
                                {isAdmin && <th className="px- py- text-right">Actions</th>}
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/">
                            {members.map((member) => (
                                <tr key={member.id} className="hover:bg-white/ transition-colors">
                                    <td className="px- py-">
                                        <div className="flex items-center gap-">
                                            <div className="w- h- rounded-full bg-blue- flex items-center justify-center font-bold text-white text-sm">
                                                {member.avatar}
                                            </div>
                                            <div>
                                                <div className="font-medium text-white">{member.name}</div>
                                                <div className="text-zinc- text-xs">{member.email}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px- py-">
                                        {isAdmin ? (
                                            <select className="bg-zinc- border border-border rounded px- py- text-xs text-white focus:ring- focus:ring-primary/ outline-none">
                                                <option>ADMIN</option>
                                                <option>ANALYST</option>
                                                <option>VIEWER</option>
                                            </select>
                                        ) : (
                                            <span className="inline-flex items-center gap-. px- py- rounded bg-purple-/ text-purple- border border-purple-/ text-xs font-medium">
                                                <Shield size={} /> {member.role}
                                            </span>
                                        )}
                                    </td>
                                    <td className="px- py-">
                                        <span className={member.status === 'active' ? 'text-emerald- font-medium' : 'text-zinc-'}>
                                            {member.status === 'active' ? 'Active' : 'Inactive'}
                                        </span>
                                    </td>
                                    {isAdmin && (
                                        <td className="px- py- text-right">
                                            <div className="flex gap- justify-end">
                                                <Button variant="ghost" size="sm" className="text-xs">
                                                    <Edit size={} />
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="text-xs"
                                                    onClick={() => handleRemoveMember(member.id)}
                                                >
                                                    <Trash size={} className="text-red-" />
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