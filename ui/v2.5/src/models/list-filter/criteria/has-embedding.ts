import {
  StringBooleanCriterion,
  StringBooleanCriterionOption,
} from "./criterion";

export const HasEmbeddingCriterionOption = new StringBooleanCriterionOption(
  "hasEmbedding",
  "has_embedding",
  () => new HasEmbeddingCriterion()
);

export class HasEmbeddingCriterion extends StringBooleanCriterion {
  constructor() {
    super(HasEmbeddingCriterionOption);
  }
}
