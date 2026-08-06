import React from "react";
import { Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { ListFilterModel } from "src/models/list-filter/filter";
import { CriterionModifier } from "src/core/generated-graphql";
import {
  SteamScoreCriterion,
  SteamScoreCriterionOption,
} from "src/models/list-filter/criteria/steam-score";
import {
  MoodsCriterion,
  MoodsCriterionOption,
} from "src/models/list-filter/criteria/moods";
import { IOptionType } from "src/models/list-filter/types";

interface ISidebarFilterProps {
  filter: ListFilterModel;
  setFilter: (filter: ListFilterModel) => void;
}

// SidebarSteamFilter filters scenes by steam score (>= value).
export const SidebarSteamFilter: React.FC<ISidebarFilterProps> = ({
  filter,
  setFilter,
}) => {
  const intl = useIntl();
  const criteria = filter.criteriaFor(SteamScoreCriterionOption.type);
  const criterion = (
    criteria.length > 0 ? criteria[0] : null
  ) as SteamScoreCriterion | null;

  const value =
    criterion && criterion.modifier === CriterionModifier.GreaterThan
      ? (criterion.value?.value ?? 0)
      : 0;

  function onChange(v: number) {
    if (v <= 0) {
      setFilter(filter.removeCriterion(SteamScoreCriterionOption.type));
      return;
    }
    const c = criterion
      ? criterion.clone()
      : SteamScoreCriterionOption.makeCriterion();
    c.modifier = CriterionModifier.GreaterThan;
    c.value = v;
    setFilter(filter.replaceCriteria(SteamScoreCriterionOption.type, [c]));
  }

  return (
    <div className="sidebar-filter">
      <Form.Label>
        <FormattedMessage id="steam_score" />
      </Form.Label>
      <Form.Control
        type="number"
        min={0}
        max={10}
        value={value}
        placeholder={intl.formatMessage({ id: "any" })}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          onChange(Number(e.currentTarget.value))
        }
      />
    </div>
  );
};

// SidebarMoodsFilter filters scenes by AI-detected moods via checkboxes.
export const SidebarMoodsFilter: React.FC<ISidebarFilterProps> = ({
  filter,
  setFilter,
}) => {
  const criteria = filter.criteriaFor(MoodsCriterionOption.type);
  const criterion = (
    criteria.length > 0 ? criteria[0] : null
  ) as MoodsCriterion | null;

  const selected = new Set(criterion?.value.items.map((i) => i.id) ?? []);

  function toggleMood(mood: string) {
    const base = criterion?.value.items ?? [];
    const items = selected.has(mood)
      ? base.filter((i) => i.id !== mood)
      : [...base, { id: mood, label: mood }];
    if (items.length === 0) {
      setFilter(filter.removeCriterion(MoodsCriterionOption.type));
      return;
    }
    const c = criterion
      ? criterion.clone()
      : MoodsCriterionOption.makeCriterion();
    c.value = { items, excluded: [] };
    setFilter(filter.replaceCriteria(MoodsCriterionOption.type, [c]));
  }

  return (
    <div className="sidebar-filter">
      <Form.Label>
        <FormattedMessage id="moods" />
      </Form.Label>
      <div>
        {((MoodsCriterionOption.options ?? []) as IOptionType[]).map((o) => {
          const mood = String(o.id);
          return (
            <Form.Check
              key={mood}
              inline
              type="checkbox"
              label={mood}
              checked={selected.has(mood)}
              onChange={() => toggleMood(mood)}
            />
          );
        })}
      </div>
    </div>
  );
};
