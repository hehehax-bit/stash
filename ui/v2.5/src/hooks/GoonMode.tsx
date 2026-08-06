import React, { createContext, useCallback, useContext } from "react";
import { ListFilterModel } from "src/models/list-filter/filter";
import * as GQL from "src/core/generated-graphql";
import { SteamScoreCriterion } from "src/models/list-filter/criteria/steam-score";

export const GOON_STEAM_FLOOR = 6;

interface IGoonModeContext {
  goonMode: boolean;
  setGoonMode: (enabled: boolean) => void;
}

const GoonModeContext = createContext<IGoonModeContext>({
  goonMode: false,
  setGoonMode: () => {},
});

export const useGoonMode = () => useContext(GoonModeContext);

// useGoonModeFilterHook returns a ListFilterModel hook that adds the steam
// score floor criterion when goon mode is active.
export function useGoonModeFilterHook(): (
  filter: ListFilterModel
) => ListFilterModel {
  const { goonMode } = useGoonMode();

  return useCallback(
    (filter: ListFilterModel) => {
      if (!goonMode) {
        return filter;
      }

      const steam = new SteamScoreCriterion();
      steam.value = GOON_STEAM_FLOOR;
      steam.modifier = GQL.CriterionModifier.GreaterThan;
      filter.criteria.push(steam);

      return filter;
    },
    [goonMode]
  );
}

interface IGoonModeProviderProps {
  children: React.ReactNode;
  goonMode: boolean;
  setGoonMode: (enabled: boolean) => void;
}

export const GoonModeProvider: React.FC<IGoonModeProviderProps> = ({
  children,
  goonMode,
  setGoonMode,
}) => (
  <GoonModeContext.Provider value={{ goonMode, setGoonMode }}>
    {children}
  </GoonModeContext.Provider>
);
