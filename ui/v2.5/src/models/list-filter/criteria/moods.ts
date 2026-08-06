import { ModifierCriterion, ModifierCriterionOption } from "./criterion";
import { CriterionModifier } from "src/core/generated-graphql";
import { ILabeledId, ILabeledValueListValue, IOptionType } from "../types";

const MOOD_OPTIONS: IOptionType[] = [
  "romantic",
  "rough",
  "goth",
  "cosplay",
  "amateur",
  "milf",
  "bdsm",
  "anal",
  "threesome",
  "taboo",
  "hardcore",
  "sensual",
  "humor",
  "solo",
  "lesbian",
  "gangbang",
  "cuckold",
  "dirty talk",
].map((m) => ({ id: m }));

export const MoodsCriterionOption = new ModifierCriterionOption({
  messageID: "moods",
  type: "moods",
  modifierOptions: [
    CriterionModifier.Includes,
    CriterionModifier.IncludesAll,
    CriterionModifier.IsNull,
    CriterionModifier.NotNull,
  ],
  defaultModifier: CriterionModifier.Includes,
  options: MOOD_OPTIONS,
  makeCriterion: () => new MoodsCriterion(),
});

export class MoodsCriterion extends ModifierCriterion<ILabeledValueListValue> {
  constructor() {
    super(MoodsCriterionOption, { items: [], excluded: [] });
  }

  public cloneValues() {
    this.value = {
      ...this.value,
      items: this.value.items.map((v: ILabeledId) => ({ ...v })),
      excluded: this.value.excluded.map((v: ILabeledId) => ({ ...v })),
    };
  }

  public toCriterionInput() {
    return {
      value: this.value.items.map((v) => v.id),
      excludes: this.value.excluded.map((v) => v.id),
      modifier: this.modifier,
    };
  }

  public getLabelValue() {
    return this.value.items.map((v) => v.label).join(", ");
  }
}
