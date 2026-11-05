export interface SchemaActionConfirmationState {
  confirmButtonText: string;
  confirmButtonLoading: boolean;
  confirmButtonDisabled: boolean;
  cancelButtonText: string;
  showCancelButton: boolean;
}

export interface SchemaActionConfirmationHandle {
  state: SchemaActionConfirmationState;
  done: () => void;
}

export type SchemaActionConfirmationAction = "confirm" | "cancel" | "close";

export interface SchemaActionConfirmationOptions {
  type: "warning";
  distinguishCancelAndClose: boolean;
  closeOnClickModal: boolean;
  closeOnPressEscape: boolean;
  beforeClose: (
    action: SchemaActionConfirmationAction,
    state: SchemaActionConfirmationState,
    done: () => void,
  ) => void;
}

export type SchemaActionConfirmationOpener = (
  message: string,
  title: string,
  options: SchemaActionConfirmationOptions,
) => Promise<unknown>;

export function requestSchemaActionConfirmation(
  message: string,
  loadingText: string,
  open: SchemaActionConfirmationOpener,
): Promise<SchemaActionConfirmationHandle | null> {
  return new Promise((resolve) => {
    let settled = false;
    let handle: SchemaActionConfirmationHandle | null = null;
    const settle = (value: SchemaActionConfirmationHandle | null) => {
      if (settled) return;
      settled = true;
      resolve(value);
    };
    void open(message, "确认操作", {
      type: "warning",
      distinguishCancelAndClose: true,
      closeOnClickModal: false,
      closeOnPressEscape: false,
      beforeClose: (action, state, done) => {
        if (handle) return;
        if (action !== "confirm") {
          done();
          settle(null);
          return;
        }
        state.confirmButtonText = loadingText;
        state.confirmButtonLoading = true;
        state.confirmButtonDisabled = true;
        state.cancelButtonText = "";
        state.showCancelButton = false;
        handle = { state, done };
        settle(handle);
      },
    }).then(() => {
      if (!handle) settle(null);
    }).catch(() => {
      if (!handle) settle(null);
    });
  });
}
