import { create } from 'zustand';

interface GlobalStore {
  isCommandPaletteOpen: boolean;
  closeOnEsc: boolean;
  openCommandPalette: () => void;
  closeCommandPalette: () => void;
  toggleCommandPalette: () => void;
}

export const useGlobalStore = create<GlobalStore>((set) => ({
  isCommandPaletteOpen: false,
  closeOnEsc: true,
  openCommandPalette: () => set({ isCommandPaletteOpen: true }),
  closeCommandPalette: () => set({ isCommandPaletteOpen: false }),
  toggleCommandPalette: () => set((state) => ({ isCommandPaletteOpen: !state.isCommandPaletteOpen })),
}));
