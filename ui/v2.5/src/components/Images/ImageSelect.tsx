import React, { useEffect, useState } from "react";
import {
  OptionProps,
  components as reactSelectComponents,
  MultiValueGenericProps,
  SingleValueProps,
} from "react-select";
import cx from "classnames";

import * as GQL from "src/core/generated-graphql";
import {
  queryFindImagesForSelect,
  queryFindImagesByIDForSelect,
} from "src/core/StashService";
import { useConfigurationContext } from "src/hooks/Config";
import { useIntl } from "react-intl";
import { defaultMaxOptionsShown } from "src/core/config";
import { ListFilterModel } from "src/models/list-filter/filter";
import {
  FilterSelectComponent,
  IFilterIDProps,
  IFilterProps,
  IFilterValueProps,
  Option as SelectOption,
  toOption,
} from "../Shared/FilterSelect";
import { useCompare } from "src/hooks/state";
import { sortByRelevance } from "src/utils/query";
import { PatchComponent, PatchFunction } from "src/patch";
import { TruncatedText } from "../Shared/TruncatedText";
import TextUtils from "src/utils/text";

export type Image = Pick<GQL.Image, "id" | "title" | "code" | "date"> & {
  paths?: Pick<GQL.ImagePathsType, "thumbnail"> | null;
  visual_files?: Array<{ path: string }> | null;
};

type Option = SelectOption<Image>;

type FindImagesResult = Awaited<
  ReturnType<typeof queryFindImagesForSelect>
>["data"]["findImages"]["images"];

function imageTitle(image: Image): string {
  if (image.title) {
    return image.title;
  }
  if (image.visual_files && image.visual_files.length > 0) {
    return TextUtils.fileNameFromPath(image.visual_files[0].path);
  }
  return "";
}

function sortImagesByRelevance(input: string, images: FindImagesResult) {
  return sortByRelevance(input, images, imageTitle, (i) => {
    return i.visual_files?.map((f) => f.path);
  });
}

const imageSelectSort = PatchFunction(
  "ImageSelect.sort",
  sortImagesByRelevance
);

const _ImageSelect: React.FC<IFilterProps & IFilterValueProps<Image>> = (
  props
) => {
  const { configuration } = useConfigurationContext();
  const intl = useIntl();
  const maxOptionsShown =
    configuration?.ui.maxOptionsShown ?? defaultMaxOptionsShown;

  async function loadImages(input: string): Promise<Option[]> {
    const filter = new ListFilterModel(GQL.FilterMode.Images);
    filter.currentPage = 1;
    filter.itemsPerPage = maxOptionsShown;
    filter.sortBy = "title";
    filter.sortDirection = GQL.SortDirectionEnum.Asc;

    filter.searchTerm = input;

    const query = await queryFindImagesForSelect(filter);
    const ret = query.data.findImages.images;

    return imageSelectSort(input, ret).map(toOption);
  }

  const ImageOption: React.FC<OptionProps<Option, boolean>> = (optionProps) => {
    let thisOptionProps = optionProps;

    const { object } = optionProps.data;

    const title = imageTitle(object);

    // if title does not match the input value but the path does, show the path
    const { inputValue } = optionProps.selectProps;
    let matchedPath: string | undefined = "";
    if (!title.toLowerCase().includes(inputValue.toLowerCase())) {
      matchedPath = object.visual_files?.find((a) =>
        a.path.toLowerCase().includes(inputValue.toLowerCase())
      )?.path;
    }

    thisOptionProps = {
      ...optionProps,
      children: (
        <span className="image-select-option">
          <span className="image-select-row">
            {object.paths?.thumbnail && (
              <img
                className="image-select-image"
                src={object.paths.thumbnail}
                loading="lazy"
              />
            )}

            <span className="image-select-details">
              <TruncatedText
                className="image-select-title"
                text={title}
                lineCount={1}
              />

              {object.date && (
                <span className="image-select-date">{object.date}</span>
              )}

              {object.code && (
                <span className="image-select-code">{object.code}</span>
              )}
            </span>
          </span>

          {matchedPath && (
            <span className="image-select-alias">{`(${matchedPath})`}</span>
          )}
        </span>
      ),
    };

    return <reactSelectComponents.Option {...thisOptionProps} />;
  };

  const ImageMultiValueLabel: React.FC<
    MultiValueGenericProps<Option, boolean>
  > = (optionProps) => {
    let thisOptionProps = optionProps;

    const { object } = optionProps.data;

    thisOptionProps = {
      ...optionProps,
      children: imageTitle(object),
    };

    return <reactSelectComponents.MultiValueLabel {...thisOptionProps} />;
  };

  const ImageValueLabel: React.FC<SingleValueProps<Option, boolean>> = (
    optionProps
  ) => {
    let thisOptionProps = optionProps;

    const { object } = optionProps.data;

    thisOptionProps = {
      ...optionProps,
      children: <>{imageTitle(object)}</>,
    };

    return <reactSelectComponents.SingleValue {...thisOptionProps} />;
  };

  return (
    <FilterSelectComponent<Image, boolean>
      {...props}
      className={cx(
        "image-select",
        {
          "image-select-active": props.active,
        },
        props.className
      )}
      loadOptions={loadImages}
      components={{
        Option: ImageOption,
        MultiValueLabel: ImageMultiValueLabel,
        SingleValue: ImageValueLabel,
      }}
      isMulti={props.isMulti ?? false}
      placeholder={
        props.noSelectionString ??
        intl.formatMessage(
          { id: "actions.select_entity" },
          {
            entityType: intl.formatMessage({
              id: props.isMulti ? "images" : "image",
            }),
          }
        )
      }
      closeMenuOnSelect={!props.isMulti}
    />
  );
};

export const ImageSelect = PatchComponent("ImageSelect", _ImageSelect);

const _ImageIDSelect: React.FC<IFilterProps & IFilterIDProps<Image>> = (
  props
) => {
  const { ids, onSelect: onSelectValues } = props;

  const [values, setValues] = useState<Image[]>([]);
  const idsChanged = useCompare(ids);

  function onSelect(items: Image[]) {
    setValues(items);
    onSelectValues?.(items);
  }

  useEffect(() => {
    async function loadObjectsByID(idsToLoad: string[]): Promise<Image[]> {
      const query = await queryFindImagesByIDForSelect(idsToLoad);
      const { images: loadedImages } = query.data.findImages;

      return loadedImages;
    }

    if (!idsChanged) {
      return;
    }

    if (!ids || ids?.length === 0) {
      setValues([]);
      return;
    }

    // load the values if we have ids and they haven't been loaded yet
    const filteredValues = values.filter((v) => ids.includes(v.id.toString()));
    if (filteredValues.length === ids.length) {
      return;
    }

    const load = async () => {
      const items = await loadObjectsByID(ids);
      setValues(items);
    };

    load();
  }, [ids, idsChanged, values]);

  return <ImageSelect {...props} values={values} onSelect={onSelect} />;
};

export const ImageIDSelect = PatchComponent("ImageIDSelect", _ImageIDSelect);
