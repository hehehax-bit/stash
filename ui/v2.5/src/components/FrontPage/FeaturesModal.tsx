import React, { useMemo, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { ModalComponent } from "src/components/Shared/Modal";
import { Icon } from "src/components/Shared/Icon";
import { faRocket, faSearch } from "@fortawesome/free-solid-svg-icons";
import { FEATURES, FEATURE_CATEGORIES } from "./featuresData";

export const FeaturesModal: React.FC<{ onClose: () => void }> = ({
  onClose,
}) => {
  const intl = useIntl();
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (q === "") return FEATURES;
    return FEATURES.filter(
      (f) =>
        f.name.toLowerCase().includes(q) ||
        f.description.toLowerCase().includes(q) ||
        f.location.toLowerCase().includes(q) ||
        f.category.toLowerCase().includes(q)
    );
  }, [query]);

  return (
    <ModalComponent
      show
      icon={faRocket}
      header={intl.formatMessage({ id: "features.heading" })}
      dialogClassName="modal-xl"
      onHide={onClose}
      cancel={{
        onClick: onClose,
        text: intl.formatMessage({ id: "actions.close" }),
        variant: "secondary",
      }}
    >
      <Form.Group>
        <Form.Control
          type="text"
          value={query}
          placeholder={intl.formatMessage({ id: "features.search" })}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
            setQuery(e.currentTarget.value)
          }
        />
      </Form.Group>

      <div className="features-guide">
        {FEATURE_CATEGORIES.map((category) => {
          const entries = filtered.filter((f) => f.category === category);
          if (entries.length === 0) return null;
          return (
            <div key={category} className="feature-category">
              <h6 className="feature-category-title">{category}</h6>
              <div className="feature-grid">
                {entries.map((f) => (
                  <div key={f.id} className="feature-card">
                    <div className="feature-card-header">
                      <span className="feature-emoji">{f.emoji}</span>
                      <strong>{f.name}</strong>
                    </div>
                    <p className="feature-description">{f.description}</p>
                    <div className="feature-location">
                      <span className="feature-label">
                        <FormattedMessage id="features.where" />:
                      </span>{" "}
                      {f.location}
                    </div>
                    <div className="feature-howto">
                      <span className="feature-label">
                        <FormattedMessage id="features.how" />:
                      </span>{" "}
                      {f.howTo}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
        {filtered.length === 0 && (
          <div className="text-muted">
            <FormattedMessage id="features.no_results" />
          </div>
        )}
      </div>
    </ModalComponent>
  );
};

export const FeaturesButton: React.FC = () => {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        <Icon icon={faSearch} /> <FormattedMessage id="features.button" />
      </Button>
      {open && <FeaturesModal onClose={() => setOpen(false)} />}
    </>
  );
};

export default FeaturesModal;
