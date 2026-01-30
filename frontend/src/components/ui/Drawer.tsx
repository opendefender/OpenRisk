import { motion, AnimatePresence } from 'framer-motion';
import { X } from 'lucide-react';
import { useEffect } from 'react';

interface DrawerProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  title?: string;
}

export const Drawer = ({ isOpen, onClose, children, title }: DrawerProps) => {
  // Lock body scroll quand ouvert
  useEffect(() => {
    if (isOpen) document.body.style.overflow = 'hidden';
    else document.body.style.overflow = 'unset';
  }, [isOpen]);

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/ Overlay sombre /}
          <motion.div
            initial={{ opacity:  }}
            animate={{ opacity:  }}
            exit={{ opacity:  }}
            onClick={onClose}
            className="fixed inset- z- bg-black/ backdrop-blur-sm"
          />
          
          {/ Panneau Lat√ral /}
          <motion.div
            initial={{ x: '%' }}
            animate={{ x:  }}
            exit={{ x: '%' }}
            transition={{ type: "spring", damping: , stiffness:  }}
            className="fixed inset-y- right- z- w-full max-w-xl bg-surface border-l border-border shadow-xl flex flex-col"
          >
            {/ Header /}
            <div className="flex items-center justify-between p- border-b border-border bg-background/">
              <h className="text-xl font-semibold text-white">{title}</h>
              <button onClick={onClose} className="p- hover:bg-white/ rounded-full transition-colors">
                <X size={} />
              </button>
            </div>

            {/ Scrollable Content /}
            <div className="flex- overflow-y-auto p- scrollbar-thin">
              {children}
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};