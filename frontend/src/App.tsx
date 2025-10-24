import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
} from "react";
import {
  Button,
  Flex,
  Heading,
  SegmentedControl,
  Separator,
  Text,
} from "@radix-ui/themes";

const MAX_TIME = 5000;
const SIZE = 512;
const TOTAL = SIZE * SIZE;

import styles from "./App.module.scss";
import {
  InfoCircledIcon,
  PauseIcon,
  PlayIcon,
  SymbolIcon,
} from "@radix-ui/react-icons";

/**
 * Start a new workflow (game) on the backend
 */
const startWorkflow = async () => {
  const response = await fetch(`${import.meta.env.VITE_BACKEND}/start`, {
    method: "POST",
  });

  const data = await response.text();

  return data;
};

/**
 * Send a splatter event to the backend
 */
const sendSplatter = async (x: number, y: number, size: number) => {
  await fetch(`${import.meta.env.VITE_BACKEND}/signal/splatter`, {
    method: "POST",
    body: JSON.stringify({ x, y, size }),
  });
};

/**
 * Pause or resume the workflow on the backend
 */
const toggleWorkflowStatus = async () => {
  await fetch(`${import.meta.env.VITE_BACKEND}/signal/toggleStatus`, {
    method: "POST",
  });
};

export default function App() {
  const [loading, setLoading] = useState(true);
  const [toggling, setToggling] = useState(false);

  const [running, setRunning] = useState(false);

  const [population, setPopulation] = useState(0);
  const [time, setTime] = useState(0);

  const [paused, setPaused] = useState(false);
  const previousPaused = useRef(false);

  const [radius, setRadius] = useState<"small" | "medium" | "large">("medium");

  const mouseDown = useRef(false);
  const tick = useRef(false);

  const board = useRef<Uint8Array>(null);
  const canvas = useRef<HTMLCanvasElement | null>(null);
  const eventSource = useRef<EventSource | null>(null);

  /**
   * Paint the current board state to the canvas
   */
  const paint = useCallback(() => {
    if (!board.current) return;

    const element = canvas.current;
    if (!element) return;

    const context = element.getContext("2d");
    if (!context) return;

    const imageData = context.createImageData(SIZE, SIZE);
    const { data } = imageData;

    // Fast pixel buffer fill
    let i = 0;
    for (let j = 0; j < TOTAL; j++) {
      data[i++] = board.current[j] ? 142 : 255; // R
      data[i++] = board.current[j] ? 140 : 255; // G
      data[i++] = board.current[j] ? 153 : 255; // B
      data[i++] = board.current[j] ? 255 : 0; // A
    }

    context.putImageData(imageData, 0, 0);
  }, []);

  /**
   * Handle window resize to scale canvas appropriately
   */
  useEffect(() => {
    const element = canvas.current;
    if (!element) return;

    const handleResize = () => {
      const multiplier =
        (Math.min(window.innerWidth, window.innerHeight) / SIZE) *
        devicePixelRatio;

      element.style.transform = `scale(${multiplier}) translate(-50%, -50%)`;
    };

    handleResize();

    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  /**
   * Initialize EventSource connection to backend
   */
  const initialize = useCallback(() => {
    eventSource.current?.close();

    board.current = new Uint8Array(TOTAL).map(() => 0);

    setTime(0);
    setPopulation(0);

    paint();

    eventSource.current = new EventSource(
      `${import.meta.env.VITE_BACKEND}/state`
    );

    eventSource.current.addEventListener("error", () => {
      eventSource.current?.close();

      setRunning(false);
      setLoading(false);
      setToggling(false);
      setPaused(false);
    });

    // Listen for state updates
    eventSource.current.addEventListener("message", (event) => {
      if (!board.current) return;

      tick.current = false;

      const data = JSON.parse(event.data) as {
        step: number;
        flipped: [number, number][] | null;
        paused: boolean;
      };

      if (data.paused !== previousPaused.current) {
        previousPaused.current = data.paused;
        setPaused(data.paused);
        setToggling(false);
      }

      if (!data.flipped) return;

      setTime(data.step);

      for (const [x, y] of data.flipped) {
        const index = y * SIZE + x;
        board.current[index] = board.current[index] ? 0 : 1;
      }

      setPopulation(board.current.reduce((a, b) => a + b, 0));

      paint();
    });

    eventSource.current.addEventListener("open", () => {
      setRunning(true);
      setLoading(false);
    });
  }, [paint]);

  /**
   * Clean up on unmount
   */
  useEffect(() => {
    initialize();

    return () => {
      eventSource.current?.close();
    };
  }, [initialize]);

  const reset = useCallback(() => {
    eventSource.current?.close();

    board.current = new Uint8Array(TOTAL).map(() => 0);
    paint();

    setRunning(false);
    setPaused(false);
    setLoading(false);
    setToggling(false);
  }, [paint]);

  const toggle = useCallback(async () => {
    setToggling(true);
    await toggleWorkflowStatus();
  }, []);

  const start = useCallback(async () => {
    setLoading(true);
    await startWorkflow();
    initialize();
  }, [initialize]);

  const handleMouseMove = useCallback(
    (event: MouseEvent) => {
      if (!mouseDown.current) return;

      if (!canvas.current) return;

      if (tick.current) return;
      tick.current = true;

      const { left, top } = canvas.current.getBoundingClientRect();
      const { clientX, clientY } = event;

      const multiplier =
        (Math.min(window.innerWidth, window.innerHeight) / SIZE) *
        devicePixelRatio;

      const x = Math.round((clientX - left) / multiplier);
      const y = Math.round((clientY - top) / multiplier);

      sendSplatter(
        x,
        y,
        radius === "small" ? 5 : radius === "medium" ? 10 : 20
      );
    },
    [radius]
  );

  useEffect(() => {
    const handleMouseUp = () => {
      mouseDown.current = false;
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [handleMouseMove]);

  const handleMouseDown = useCallback(
    (event: ReactMouseEvent) => {
      mouseDown.current = true;
      handleMouseMove(event.nativeEvent);
    },
    [handleMouseMove]
  );

  return (
    <Flex align="center" direction="column" justify="center">
      <canvas
        className={styles.canvas}
        ref={canvas}
        width={SIZE}
        height={SIZE}
        onMouseDown={handleMouseDown}
      />
      <Flex
        gap="2"
        py="2"
        px="3"
        direction="column"
        className={styles.panel}
        position="fixed"
        bottom="4"
        left="4"
        width="240px"
      >
        <Flex align="center" justify="between">
          <Text size="2" color="gray">
            Population
          </Text>
          <Heading size="3">{population.toLocaleString()}</Heading>
        </Flex>
        <Separator size="4" />
        <Flex align="center" justify="between">
          <Text size="2" color="gray">
            Time
          </Text>
          <Heading size="3">{time.toLocaleString()}</Heading>
        </Flex>
      </Flex>
      <Flex position="fixed" top="4" left="4" gap="3" direction="column">
        <Flex gap="2" p="2" className={styles.panel} align="center">
          {running ? (
            <Flex align="center" gap="2">
              {time < MAX_TIME && (
                <Button
                  onClick={() => toggle()}
                  loading={toggling}
                  disabled={toggling}
                >
                  {!paused && <PauseIcon />}
                  {paused ? "Resume" : "Pause"}
                  {paused && <PlayIcon />}
                </Button>
              )}
              <Button color="gray" variant="soft" onClick={() => reset()}>
                <SymbolIcon />
                Reset
              </Button>
            </Flex>
          ) : (
            <Button
              onClick={() => start()}
              loading={loading}
              disabled={loading}
            >
              Start
              <PlayIcon />
            </Button>
          )}
          <Separator orientation="vertical" />
          <Flex align="center" gap="3" pl="1">
            <Text size="2" color="gray">
              Splatter size
            </Text>
            <SegmentedControl.Root
              disabled={!running || time >= MAX_TIME}
              value={radius}
              onValueChange={(value) =>
                setRadius(value as "small" | "medium" | "large")
              }
            >
              <SegmentedControl.Item value="small">Small</SegmentedControl.Item>
              <SegmentedControl.Item value="medium">
                Medium
              </SegmentedControl.Item>
              <SegmentedControl.Item value="large">Large</SegmentedControl.Item>
            </SegmentedControl.Root>
          </Flex>
        </Flex>
        <Text size="2" color="gray">
          <Flex align="center" gap="3" as="span">
            <InfoCircledIcon />
            You can click on the canvas to splatter cells
          </Flex>
        </Text>
      </Flex>
    </Flex>
  );
}
