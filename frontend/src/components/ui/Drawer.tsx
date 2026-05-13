import { motion, AnimatePresence } from 'framer-motion';
import { X } from 'lucide-react';
import { useEffect } from 'react';

interface DrawerProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  title?: string;
  widthClassName?: string;
}

export const Drawer = ({ isOpen, onClose, children, title, widthClassName = 'max-w-2xl' }: DrawerProps) => {
  // Lock body scroll quand ouvert
  useEffect(() => {
    if (isOpen) document.body.style.overflow = 'hidden';
    else document.body.style.overflow = 'unset';
  }, [isOpen]);

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Overlay sombre */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
          />
          
          {/* Panneau Latéral */}
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: "spring", damping: 25, stiffness: 200 }}
            className={`fixed inset-y-0 right-0 z-50 w-full ${widthClassName} bg-surface border-l border-border shadow-2xl flex flex-col`}
          >
            {/* Header */}
            <div className="flex items-center justify-between p-6 border-b border-border bg-background/50">
              <h2 className="text-xl font-semibold text-white">{title}</h2>
              <button onClick={onClose} className="p-2 hover:bg-white/10 rounded-full transition-colors">
                <X size={20} />
              </button>
            </div>

            {/* Scrollable Content */}
            <div className="flex-1 overflow-y-auto p-6 scrollbar-thin">
              {children}
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};