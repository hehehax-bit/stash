import { NumberCriterion, NumberCriterionOption } from "./criterion";

export const SteamScoreCriterionOption = new NumberCriterionOption(
  "steam_score",
  "steam_score"
);

export class SteamScoreCriterion extends NumberCriterion {
  constructor() {
    super(SteamScoreCriterionOption);
  }
}
